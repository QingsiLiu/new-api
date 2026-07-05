package controller

import (
	"errors"
	"net/http"
	"sort"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
)

func GetGroups(c *gin.Context) {
	groupNames := make([]string, 0)
	for groupName := range ratio_setting.GetGroupRatioCopy() {
		groupNames = append(groupNames, groupName)
	}
	sort.Strings(groupNames)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    groupNames,
		"groups":  model.ResolveGroupDisplays(groupNames),
	})
}

func GetUserGroups(c *gin.Context) {
	usableGroups := make(map[string]map[string]interface{})
	userGroup := ""
	userId := c.GetInt("id")
	userGroup, _ = model.GetUserGroup(userId, false)
	userUsableGroups := service.GetUserUsableGroups(userGroup)
	for groupName, _ := range ratio_setting.GetGroupRatioCopy() {
		// UserUsableGroups contains the groups that the user can use
		if desc, ok := userUsableGroups[groupName]; ok {
			usableGroups[groupName] = map[string]interface{}{
				"ratio":        service.GetUserGroupRatio(userGroup, groupName),
				"desc":         desc,
				"display_name": model.ResolveGroupDisplay(groupName),
			}
		}
	}
	if _, ok := userUsableGroups["auto"]; ok {
		usableGroups["auto"] = map[string]interface{}{
			"ratio":        "自动",
			"desc":         setting.GetUsableGroupDescription("auto"),
			"display_name": model.ResolveGroupDisplay("auto"),
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    usableGroups,
	})
}

func ListGroupRegistry(c *gin.Context) {
	groups, err := model.ListGroupRegistryViews()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, groups)
}

func CreateGroupRegistry(c *gin.Context) {
	var req model.GroupRegistryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	group, err := model.CreateGroupRegistry(req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, model.BuildGroupRegistryView(group))
}

func UpdateGroupRegistry(c *gin.Context) {
	var req model.GroupRegistryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	group, err := model.UpdateGroupRegistry(c.Param("code"), req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, model.BuildGroupRegistryView(group))
}

func DeleteGroupRegistry(c *gin.Context) {
	err := model.DeleteGroupRegistry(c.Param("code"))
	if err == nil {
		common.ApiSuccess(c, nil)
		return
	}
	var inUse *model.GroupRegistryInUseError
	if errors.As(err, &inUse) {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": inUse.Error(),
			"data":    inUse.References,
		})
		return
	}
	common.ApiError(c, err)
}
