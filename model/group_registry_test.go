package model

import (
	"math"
	"regexp"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/require"
)

func TestResolveGroupDisplayFallsBackToCode(t *testing.T) {
	truncateTables(t)

	require.Equal(t, "missing_group", ResolveGroupDisplay("missing_group"))
}

func TestResolveGroupDisplayUsesRegistryDisplayName(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.Create(&GroupRegistry{
		Code:        "vip",
		DisplayName: "VIP Display",
	}).Error)
	InvalidateGroupRegistryCache()

	require.Equal(t, "VIP Display", ResolveGroupDisplay("vip"))
}

func TestResolveGroupDisplayBatchFallsBackPerCode(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.Create(&GroupRegistry{
		Code:        "default",
		DisplayName: "Default Display",
	}).Error)
	InvalidateGroupRegistryCache()

	displays := ResolveGroupDisplayBatch([]string{"default", "unknown"})

	require.Equal(t, "Default Display", displays["default"])
	require.Equal(t, "unknown", displays["unknown"])
}

func TestSplitAndResolveChannelGroups(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.Create(&GroupRegistry{
		Code:        "default",
		DisplayName: "Default Display",
	}).Error)
	InvalidateGroupRegistryCache()

	groups := SplitAndResolveChannelGroups(" default, unknown ,,")

	require.Equal(t, []GroupDisplay{
		{Code: "default", DisplayName: "Default Display"},
		{Code: "unknown", DisplayName: "unknown"},
	}, groups)
}

func TestReconcileGroupRegistryCollectsAllSourcesAndIsIdempotent(t *testing.T) {
	truncateTables(t)
	originalGroupRatio := ratio_setting.GroupRatio2JSONString()
	originalGroupGroupRatio := ratio_setting.GroupGroupRatio2JSONString()
	originalUserUsableGroups := setting.UserUsableGroups2JSONString()
	originalAutoGroups := setting.AutoGroups2JsonString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatio))
		require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(originalGroupGroupRatio))
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalUserUsableGroups))
		require.NoError(t, setting.UpdateAutoGroupsByJsonString(originalAutoGroups))
		InvalidateGroupRegistryCache()
	})

	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"ratio_group":1}`))
	require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(`{"outer_group":{"inner_group":0.8}}`))
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"usable_group":"desc"}`))
	require.NoError(t, setting.UpdateAutoGroupsByJsonString(`["auto_source_group"]`))
	require.NoError(t, DB.Create(&User{Username: "user-source", Password: "password123", Group: "user_group"}).Error)
	require.NoError(t, DB.Create(&Token{Name: "token-source", Key: "sk-source", UserId: 1, Group: "token_group"}).Error)
	require.NoError(t, DB.Create(&Channel{Name: "channel-source", Key: "key", Group: "channel_a, channel_b"}).Error)
	require.NoError(t, DB.Create(&Ability{Group: "ability_group", Model: "gpt-test", ChannelId: 1, Enabled: true}).Error)

	require.NoError(t, ReconcileGroupRegistry())
	require.NoError(t, ReconcileGroupRegistry())

	expectedCodes := []string{
		"ratio_group",
		"outer_group",
		"inner_group",
		"usable_group",
		"user_group",
		"token_group",
		"channel_a",
		"channel_b",
		"ability_group",
		"auto_source_group",
		"default",
	}
	for _, code := range expectedCodes {
		group, err := GetGroupRegistryByCode(code)
		require.NoError(t, err, code)
		require.Equal(t, code, group.Code)
		require.Equal(t, code, group.DisplayName)
	}

	var count int64
	require.NoError(t, DB.Model(&GroupRegistry{}).Where("code = ?", "ratio_group").Count(&count).Error)
	require.EqualValues(t, 1, count)

	defaultGroup, err := GetGroupRegistryByCode("default")
	require.NoError(t, err)
	require.True(t, defaultGroup.IsReserved)
}

func TestGenerateGroupRegistryCodeFormatAndCollisionRetry(t *testing.T) {
	truncateTables(t)
	originalGenerator := groupRegistryCodeGenerator
	t.Cleanup(func() {
		groupRegistryCodeGenerator = originalGenerator
	})
	require.NoError(t, DB.Create(&GroupRegistry{Code: "grp_aaaaaaaa", DisplayName: "taken"}).Error)
	InvalidateGroupRegistryCache()

	calls := 0
	groupRegistryCodeGenerator = func() string {
		calls++
		if calls == 1 {
			return "grp_aaaaaaaa"
		}
		return "grp_bbbbbbbb"
	}

	code, err := generateGroupRegistryCode()

	require.NoError(t, err)
	require.Equal(t, "grp_bbbbbbbb", code)
	require.Equal(t, 2, calls)
	require.Regexp(t, regexp.MustCompile(`^grp_[a-z0-9]{8}$`), code)
}

func TestCreateGroupRegistryGeneratesHiddenCodeAndSeedsGroupRatio(t *testing.T) {
	truncateTables(t)
	originalGroupRatio := ratio_setting.GroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatio))
	})

	created, err := CreateGroupRegistry(GroupRegistryRequest{
		DisplayName: "Customer Plan",
		Description: stringPtr("Customer plan desc"),
		Ratio:       floatPtr(1.25),
		UserUsable:  boolPtr(true),
		Sort:        intPtr(10),
	})

	require.NoError(t, err)
	require.Regexp(t, regexp.MustCompile(`^grp_[a-z0-9]{8}$`), created.Code)
	require.Equal(t, "Customer Plan", created.DisplayName)
	require.Equal(t, "Customer plan desc", created.Description)
	require.False(t, created.IsReserved)
	require.Equal(t, 10, created.Sort)
	require.Equal(t, 1.25, ratio_setting.GetGroupRatio(created.Code))
	require.Equal(t, "Customer plan desc", setting.GetUserUsableGroupsCopy()[created.Code])
}

func TestCreateGroupRegistryDoesNotLeaveRegistryRowWhenSeedFails(t *testing.T) {
	truncateTables(t)
	originalGenerator := groupRegistryCodeGenerator
	t.Cleanup(func() {
		groupRegistryCodeGenerator = originalGenerator
	})
	groupRegistryCodeGenerator = func() string {
		return "grp_badseed1"
	}

	created, err := CreateGroupRegistry(GroupRegistryRequest{
		DisplayName: "Bad Seed",
		Ratio:       floatPtr(math.NaN()),
	})

	require.Error(t, err)
	require.Nil(t, created)
	var count int64
	require.NoError(t, DB.Model(&GroupRegistry{}).Where("code = ?", "grp_badseed1").Count(&count).Error)
	require.EqualValues(t, 0, count)
}

func TestUpdateGroupRegistryRenamesWithoutChangingCode(t *testing.T) {
	truncateTables(t)
	originalGroupRatio := ratio_setting.GroupRatio2JSONString()
	originalUserUsableGroups := setting.UserUsableGroups2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatio))
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalUserUsableGroups))
	})
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"legacy":1}`))
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"legacy":"old desc"}`))
	require.NoError(t, DB.Create(&GroupRegistry{Code: "legacy", DisplayName: "Legacy"}).Error)
	InvalidateGroupRegistryCache()

	updated, err := UpdateGroupRegistry("legacy", GroupRegistryRequest{
		Code:        "attempted-code-change",
		DisplayName: "Renamed",
		Description: stringPtr("new desc"),
		Ratio:       floatPtr(0.9),
		UserUsable:  boolPtr(true),
	})

	require.NoError(t, err)
	require.Equal(t, "legacy", updated.Code)
	require.Equal(t, "Renamed", updated.DisplayName)
	require.Equal(t, 0.9, ratio_setting.GetGroupRatio("legacy"))
	require.Equal(t, "new desc", setting.GetUserUsableGroupsCopy()["legacy"])
}

func TestUpdateGroupRegistryRestoresDisplayWhenSeedFails(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.Create(&GroupRegistry{
		Code:        "legacy",
		DisplayName: "Legacy",
		Description: "old desc",
	}).Error)
	InvalidateGroupRegistryCache()

	updated, err := UpdateGroupRegistry("legacy", GroupRegistryRequest{
		DisplayName: "Renamed",
		Ratio:       floatPtr(math.NaN()),
	})

	require.Error(t, err)
	require.Nil(t, updated)
	group, err := GetGroupRegistryByCode("legacy")
	require.NoError(t, err)
	require.Equal(t, "Legacy", group.DisplayName)
	require.Equal(t, "old desc", group.Description)
}

func TestUpdateGroupRegistryDisplayOnlyKeepsDescription(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.Create(&GroupRegistry{
		Code:        "legacy",
		DisplayName: "Legacy",
		Description: "kept desc",
	}).Error)
	InvalidateGroupRegistryCache()

	updated, err := UpdateGroupRegistry("legacy", GroupRegistryRequest{
		DisplayName: "Renamed",
	})

	require.NoError(t, err)
	require.Equal(t, "legacy", updated.Code)
	require.Equal(t, "Renamed", updated.DisplayName)
	require.Equal(t, "kept desc", updated.Description)
}

func TestDeleteGroupRegistryRejectsReferencedCode(t *testing.T) {
	truncateTables(t)
	originalGroupRatio := ratio_setting.GroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatio))
	})
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"busy":1}`))
	require.NoError(t, DB.Create(&GroupRegistry{Code: "busy", DisplayName: "Busy"}).Error)
	InvalidateGroupRegistryCache()

	err := DeleteGroupRegistry("busy")

	var inUse *GroupRegistryInUseError
	require.ErrorAs(t, err, &inUse)
	require.Equal(t, 1, inUse.References["group_ratio"])
}

func TestDeleteGroupRegistryRejectsCodeReferencedOnlyByChannelMember(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.Create(&GroupRegistry{Code: "target", DisplayName: "Target"}).Error)
	require.NoError(t, DB.Create(&Channel{Name: "channel-member-source", Key: "key", Group: "other, target, target_suffix"}).Error)
	InvalidateGroupRegistryCache()

	err := DeleteGroupRegistry("target")

	var inUse *GroupRegistryInUseError
	require.ErrorAs(t, err, &inUse)
	require.Equal(t, 1, inUse.References["channels"])
}

func TestDeleteGroupRegistryDeletesUnreferencedNonReservedCode(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.Create(&GroupRegistry{Code: "orphan", DisplayName: "Orphan"}).Error)
	InvalidateGroupRegistryCache()

	require.NoError(t, DeleteGroupRegistry("orphan"))

	_, err := GetGroupRegistryByCode("orphan")
	require.Error(t, err)
}

func floatPtr(value float64) *float64 {
	return &value
}

func boolPtr(value bool) *bool {
	return &value
}

func intPtr(value int) *int {
	return &value
}

func stringPtr(value string) *string {
	return &value
}

func marshalJSONForTest(t *testing.T, value any) string {
	t.Helper()
	data, err := common.Marshal(value)
	require.NoError(t, err)
	return string(data)
}
