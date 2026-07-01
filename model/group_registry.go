package model

import (
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	groupRegistryCodePrefix   = "grp_"
	groupRegistryCodeLength   = 8
	groupRegistryCodeMaxRetry = 16
)

type GroupRegistry struct {
	Id          int    `json:"id" gorm:"primaryKey"`
	Code        string `json:"code" gorm:"type:varchar(64);not null;uniqueIndex;index"`
	DisplayName string `json:"display_name" gorm:"type:varchar(128);not null;index"`
	Description string `json:"description" gorm:"type:varchar(255);default:''"`
	IsReserved  bool   `json:"is_reserved" gorm:"default:false;index"`
	Sort        int    `json:"sort" gorm:"default:0;index"`
	CreatedTime int64  `json:"created_time" gorm:"autoCreateTime;column:created_time"`
	UpdatedTime int64  `json:"updated_time" gorm:"autoUpdateTime;column:updated_time"`
}

func (GroupRegistry) TableName() string {
	return "groups"
}

type GroupDisplay struct {
	Code        string `json:"code"`
	DisplayName string `json:"display_name"`
}

type GroupRegistryRequest struct {
	Code        string   `json:"code,omitempty"`
	DisplayName string   `json:"display_name"`
	Description *string  `json:"description,omitempty"`
	Ratio       *float64 `json:"ratio,omitempty"`
	UserUsable  *bool    `json:"user_usable,omitempty"`
	Sort        *int     `json:"sort,omitempty"`
}

type GroupRegistryInUseError struct {
	Code       string         `json:"code"`
	References map[string]int `json:"references"`
}

func (e *GroupRegistryInUseError) Error() string {
	return "group " + e.Code + " is still referenced"
}

type GroupRegistryView struct {
	Code        string  `json:"code"`
	DisplayName string  `json:"display_name"`
	Description string  `json:"description"`
	Ratio       float64 `json:"ratio"`
	UserUsable  bool    `json:"user_usable"`
	IsReserved  bool    `json:"is_reserved"`
	Sort        int     `json:"sort"`
}

var (
	groupRegistryCacheLock     sync.RWMutex
	groupRegistryCache         = map[string]GroupRegistry{}
	groupRegistryCacheTime     time.Time
	groupRegistryCodeGenerator = randomGroupRegistryCode
)

func InvalidateGroupRegistryCache() {
	groupRegistryCacheLock.Lock()
	defer groupRegistryCacheLock.Unlock()
	groupRegistryCache = map[string]GroupRegistry{}
	groupRegistryCacheTime = time.Time{}
}

func loadGroupRegistryCacheLocked() error {
	var groups []GroupRegistry
	if err := DB.Order("sort asc, id asc").Find(&groups).Error; err != nil {
		return err
	}
	nextCache := make(map[string]GroupRegistry, len(groups))
	for _, group := range groups {
		nextCache[group.Code] = group
	}
	groupRegistryCache = nextCache
	groupRegistryCacheTime = time.Now()
	return nil
}

func ensureGroupRegistryCache() error {
	groupRegistryCacheLock.RLock()
	cacheReady := len(groupRegistryCache) > 0
	groupRegistryCacheLock.RUnlock()
	if cacheReady {
		return nil
	}

	groupRegistryCacheLock.Lock()
	defer groupRegistryCacheLock.Unlock()
	if len(groupRegistryCache) > 0 {
		return nil
	}
	return loadGroupRegistryCacheLocked()
}

func GetGroupRegistryByCode(code string) (*GroupRegistry, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, gorm.ErrRecordNotFound
	}
	if err := ensureGroupRegistryCache(); err == nil {
		groupRegistryCacheLock.RLock()
		if group, ok := groupRegistryCache[code]; ok {
			groupRegistryCacheLock.RUnlock()
			return &group, nil
		}
		groupRegistryCacheLock.RUnlock()
	}

	var group GroupRegistry
	if err := DB.Where("code = ?", code).First(&group).Error; err != nil {
		return nil, err
	}
	groupRegistryCacheLock.Lock()
	groupRegistryCache[group.Code] = group
	groupRegistryCacheLock.Unlock()
	return &group, nil
}

func ResolveGroupDisplay(code string) string {
	code = strings.TrimSpace(code)
	if code == "" {
		return ""
	}
	group, err := GetGroupRegistryByCode(code)
	if err != nil || strings.TrimSpace(group.DisplayName) == "" {
		return code
	}
	return group.DisplayName
}

func ResolveGroupDisplayBatch(codes []string) map[string]string {
	displays := make(map[string]string, len(codes))
	for _, code := range codes {
		trimmed := strings.TrimSpace(code)
		if trimmed == "" {
			continue
		}
		displays[trimmed] = ResolveGroupDisplay(trimmed)
	}
	return displays
}

func SplitAndResolveChannelGroups(csv string) []GroupDisplay {
	parts := strings.Split(csv, ",")
	resolved := make([]GroupDisplay, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		code := strings.TrimSpace(part)
		if code == "" {
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		resolved = append(resolved, GroupDisplay{
			Code:        code,
			DisplayName: ResolveGroupDisplay(code),
		})
	}
	return resolved
}

func ResolveGroupDisplays(codes []string) []GroupDisplay {
	if len(codes) == 0 {
		return []GroupDisplay{}
	}
	resolved := make([]GroupDisplay, 0, len(codes))
	for _, code := range codes {
		code = strings.TrimSpace(code)
		if code == "" {
			continue
		}
		resolved = append(resolved, GroupDisplay{
			Code:        code,
			DisplayName: ResolveGroupDisplay(code),
		})
	}
	return resolved
}

func randomGroupRegistryCode() string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	buf := make([]byte, groupRegistryCodeLength)
	for i := range buf {
		buf[i] = alphabet[rand.Intn(len(alphabet))]
	}
	return groupRegistryCodePrefix + string(buf)
}

func generateGroupRegistryCode() (string, error) {
	for i := 0; i < groupRegistryCodeMaxRetry; i++ {
		code := groupRegistryCodeGenerator()
		if !isGeneratedGroupCode(code) {
			continue
		}
		if _, err := GetGroupRegistryByCode(code); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return code, nil
			}
		}
	}
	return "", fmt.Errorf("failed to generate unique group code after %d attempts", groupRegistryCodeMaxRetry)
}

func isGeneratedGroupCode(code string) bool {
	if len(code) != len(groupRegistryCodePrefix)+groupRegistryCodeLength {
		return false
	}
	if !strings.HasPrefix(code, groupRegistryCodePrefix) {
		return false
	}
	for _, c := range strings.TrimPrefix(code, groupRegistryCodePrefix) {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')) {
			return false
		}
	}
	return code != "auto" && code != "default" && code != "vip"
}

func collectExistingGroupCodes() ([]string, error) {
	codes := make(map[string]struct{})
	addCode := func(code string) {
		code = strings.TrimSpace(code)
		if code == "" {
			return
		}
		codes[code] = struct{}{}
	}

	for code := range ratio_setting.GetGroupRatioCopy() {
		addCode(code)
	}
	for outer, innerMap := range ratio_setting.GetGroupGroupRatioCopy() {
		addCode(outer)
		for inner := range innerMap {
			addCode(inner)
		}
	}
	for code := range setting.GetUserUsableGroupsCopy() {
		addCode(code)
	}

	userGroups, err := selectDistinctGroupColumn(&User{})
	if err != nil {
		return nil, err
	}
	for _, code := range userGroups {
		addCode(code)
	}

	tokenGroups, err := selectDistinctGroupColumn(&Token{})
	if err != nil {
		return nil, err
	}
	for _, code := range tokenGroups {
		addCode(code)
	}

	channelGroups, err := selectDistinctGroupColumn(&Channel{})
	if err != nil {
		return nil, err
	}
	for _, csv := range channelGroups {
		for _, group := range strings.Split(csv, ",") {
			addCode(group)
		}
	}

	abilityGroups, err := selectDistinctGroupColumn(&Ability{})
	if err != nil {
		return nil, err
	}
	for _, code := range abilityGroups {
		addCode(code)
	}

	for _, code := range setting.GetAutoGroups() {
		addCode(code)
	}
	addCode("default")

	resolved := make([]string, 0, len(codes))
	for code := range codes {
		resolved = append(resolved, code)
	}
	return resolved, nil
}

func selectDistinctGroupColumn(modelValue any) ([]string, error) {
	var groups []string
	err := DB.Model(modelValue).Select(commonGroupCol).Distinct().Find(&groups).Error
	return groups, err
}

func ensureGroupRegistryEntry(code string, reserved bool) error {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil
	}

	group, err := GetGroupRegistryByCode(code)
	if err == nil {
		if group.DisplayName == "" {
			group.DisplayName = code
			group.IsReserved = group.IsReserved || reserved
			if err := DB.Save(group).Error; err != nil {
				return err
			}
			InvalidateGroupRegistryCache()
		} else if reserved && !group.IsReserved {
			if err := DB.Model(&GroupRegistry{}).Where("code = ?", code).Update("is_reserved", true).Error; err != nil {
				return err
			}
			InvalidateGroupRegistryCache()
		}
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	entry := &GroupRegistry{
		Code:        code,
		DisplayName: code,
		IsReserved:  reserved,
	}
	if err := DB.Clauses(clause.OnConflict{DoNothing: true}).Create(entry).Error; err != nil {
		return err
	}
	InvalidateGroupRegistryCache()
	return nil
}

func CreateGroupRegistry(req GroupRegistryRequest) (*GroupRegistry, error) {
	displayName := strings.TrimSpace(req.DisplayName)
	if displayName == "" {
		return nil, errors.New("display_name is required")
	}
	code, err := generateGroupRegistryCode()
	if err != nil {
		return nil, err
	}
	group := &GroupRegistry{
		Code:        code,
		DisplayName: displayName,
	}
	if req.Description != nil {
		group.Description = strings.TrimSpace(*req.Description)
	}
	if req.Sort != nil {
		group.Sort = *req.Sort
	}
	if err := DB.Create(group).Error; err != nil {
		return nil, err
	}
	if req.Ratio == nil {
		defaultRatio := 1.0
		req.Ratio = &defaultRatio
	}
	if err := seedGroupRegistrySettings(code, req); err != nil {
		_ = DB.Where("code = ?", code).Delete(&GroupRegistry{}).Error
		InvalidateGroupRegistryCache()
		return nil, err
	}
	InvalidateGroupRegistryCache()
	return group, nil
}

func UpdateGroupRegistry(code string, req GroupRegistryRequest) (*GroupRegistry, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, errors.New("group code is required")
	}
	group, err := GetGroupRegistryByCode(code)
	if err != nil {
		return nil, err
	}
	previousGroup := *group
	if displayName := strings.TrimSpace(req.DisplayName); displayName != "" {
		group.DisplayName = displayName
	}
	if req.Description != nil {
		group.Description = strings.TrimSpace(*req.Description)
	}
	if req.Sort != nil {
		group.Sort = *req.Sort
	}
	if err := DB.Save(group).Error; err != nil {
		return nil, err
	}
	if err := seedGroupRegistrySettings(code, req); err != nil {
		_ = DB.Save(&previousGroup).Error
		InvalidateGroupRegistryCache()
		return nil, err
	}
	InvalidateGroupRegistryCache()
	return group, nil
}

func seedGroupRegistrySettings(code string, req GroupRegistryRequest) error {
	if req.Ratio != nil {
		previousGroupRatio := ratio_setting.GroupRatio2JSONString()
		groupRatio := ratio_setting.GetGroupRatioCopy()
		groupRatio[code] = *req.Ratio
		jsonStr, err := marshalGroupRegistryJSON(groupRatio)
		if err != nil {
			return err
		}
		if err := ratio_setting.UpdateGroupRatioByJSONString(jsonStr); err != nil {
			return err
		}
		if err := persistGroupRegistryOption("GroupRatio", ratio_setting.GroupRatio2JSONString()); err != nil {
			_ = ratio_setting.UpdateGroupRatioByJSONString(previousGroupRatio)
			return err
		}
	}
	if req.UserUsable != nil {
		previousUserUsableGroups := setting.UserUsableGroups2JSONString()
		userUsableGroups := setting.GetUserUsableGroupsCopy()
		if *req.UserUsable {
			desc := strings.TrimSpace(groupRegistryRequestDescription(req))
			if desc == "" {
				desc = strings.TrimSpace(req.DisplayName)
			}
			userUsableGroups[code] = desc
		} else {
			delete(userUsableGroups, code)
		}
		jsonStr, err := marshalGroupRegistryJSON(userUsableGroups)
		if err != nil {
			return err
		}
		if err := setting.UpdateUserUsableGroupsByJSONString(jsonStr); err != nil {
			return err
		}
		if err := persistGroupRegistryOption("UserUsableGroups", setting.UserUsableGroups2JSONString()); err != nil {
			_ = setting.UpdateUserUsableGroupsByJSONString(previousUserUsableGroups)
			return err
		}
	}
	return nil
}

func groupRegistryRequestDescription(req GroupRegistryRequest) string {
	if req.Description == nil {
		return ""
	}
	return *req.Description
}

func persistGroupRegistryOption(key string, value string) error {
	if DB == nil {
		return nil
	}
	if !DB.Migrator().HasTable(&Option{}) {
		return nil
	}
	option := Option{Key: key}
	if err := DB.FirstOrCreate(&option, Option{Key: key}).Error; err != nil {
		return err
	}
	option.Value = value
	if err := DB.Save(&option).Error; err != nil {
		return err
	}
	common.OptionMapRWMutex.Lock()
	if common.OptionMap == nil {
		common.OptionMap = make(map[string]string)
	}
	common.OptionMap[key] = value
	common.OptionMapRWMutex.Unlock()
	return nil
}

func marshalGroupRegistryJSON(value any) (string, error) {
	data, err := common.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func DeleteGroupRegistry(code string) error {
	code = strings.TrimSpace(code)
	if code == "" {
		return errors.New("group code is required")
	}
	group, err := GetGroupRegistryByCode(code)
	if err != nil {
		return err
	}
	if group.IsReserved {
		return errors.New("reserved group cannot be deleted")
	}
	references := CountGroupRegistryReferences(code)
	if len(references) > 0 {
		return &GroupRegistryInUseError{Code: code, References: references}
	}
	if err := DB.Where("code = ?", code).Delete(&GroupRegistry{}).Error; err != nil {
		return err
	}
	InvalidateGroupRegistryCache()
	return nil
}

func CountGroupRegistryReferences(code string) map[string]int {
	references := map[string]int{}
	if _, ok := ratio_setting.GetGroupRatioCopy()[code]; ok {
		references["group_ratio"] = 1
	}
	for outer, inner := range ratio_setting.GetGroupGroupRatioCopy() {
		if outer == code {
			references["group_group_ratio_outer"]++
		}
		if _, ok := inner[code]; ok {
			references["group_group_ratio_inner"]++
		}
	}
	if _, ok := setting.GetUserUsableGroupsCopy()[code]; ok {
		references["user_usable_groups"] = 1
	}

	countGroupRegistryRows(code, &User{}, "users", references)
	countGroupRegistryRows(code, &Token{}, "tokens", references)
	countGroupRegistryRows(code, &Ability{}, "abilities", references)

	channelGroups, err := selectDistinctGroupColumn(&Channel{})
	if err == nil {
		for _, csv := range channelGroups {
			for _, group := range strings.Split(csv, ",") {
				if strings.TrimSpace(group) == code {
					references["channels"]++
					break
				}
			}
		}
	}
	for key, value := range references {
		if value <= 0 {
			delete(references, key)
		}
	}
	return references
}

func countGroupRegistryRows(code string, modelValue any, key string, references map[string]int) {
	var count int64
	if err := DB.Model(modelValue).Where(commonGroupCol+" = ?", code).Count(&count).Error; err == nil && count > 0 {
		references[key] = int(count)
	}
}

func ListGroupRegistry() ([]GroupRegistry, error) {
	if err := ReconcileGroupRegistry(); err != nil {
		return nil, err
	}
	var groups []GroupRegistry
	err := DB.Order("sort asc, id asc").Find(&groups).Error
	return groups, err
}

func ListGroupRegistryViews() ([]GroupRegistryView, error) {
	groups, err := ListGroupRegistry()
	if err != nil {
		return nil, err
	}
	views := make([]GroupRegistryView, 0, len(groups))
	for _, group := range groups {
		views = append(views, BuildGroupRegistryView(&group))
	}
	return views, nil
}

func BuildGroupRegistryView(group *GroupRegistry) GroupRegistryView {
	if group == nil {
		return GroupRegistryView{}
	}
	userUsableGroups := setting.GetUserUsableGroupsCopy()
	desc, userUsable := userUsableGroups[group.Code]
	if strings.TrimSpace(desc) == "" {
		desc = group.Description
	}
	ratio := 0.0
	if groupRatio, ok := ratio_setting.GetGroupRatioCopy()[group.Code]; ok {
		ratio = groupRatio
	}
	return GroupRegistryView{
		Code:        group.Code,
		DisplayName: ResolveGroupDisplay(group.Code),
		Description: desc,
		Ratio:       ratio,
		UserUsable:  userUsable,
		IsReserved:  group.IsReserved,
		Sort:        group.Sort,
	}
}

func GroupRegistryRatioString(code string) string {
	ratio := BuildGroupRegistryView(&GroupRegistry{Code: code}).Ratio
	if ratio == 0 {
		return ""
	}
	return strconv.FormatFloat(ratio, 'f', -1, 64)
}

func ReconcileGroupRegistry() error {
	codes, err := collectExistingGroupCodes()
	if err != nil {
		return err
	}
	for _, code := range codes {
		reserved := code == "auto" || code == "default"
		if err := ensureGroupRegistryEntry(code, reserved); err != nil {
			return err
		}
	}
	InvalidateGroupRegistryCache()
	return loadGroupRegistryCacheLockedWithLock()
}

func loadGroupRegistryCacheLockedWithLock() error {
	groupRegistryCacheLock.Lock()
	defer groupRegistryCacheLock.Unlock()
	return loadGroupRegistryCacheLocked()
}

func InitGroupRegistry() error {
	if err := DB.AutoMigrate(&GroupRegistry{}); err != nil {
		return err
	}
	return ReconcileGroupRegistry()
}
