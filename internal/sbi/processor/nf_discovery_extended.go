package processor

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	compatnrf "github.com/free5gc/nrf/internal/compat/nrf"
	"github.com/free5gc/openapi/models"
)

const nrfR18SupportedFeatures = "800010000"

type extendedDiscoveryCriteria struct {
	nwdafEvents         []models.NwdafEvent
	mlAnalyticsInfoList []compatnrf.MLAnalyticsInfo
	internalGroupID     string
	mlModelStorageInd   *bool
	dataStorageInd      *bool
}

func parseExtendedDiscoveryCriteria(
	values url.Values,
) (extendedDiscoveryCriteria, error) {
	var criteria extendedDiscoveryCriteria
	if raw, present := values["nwdaf-event-list"]; present {
		if len(raw) != 1 || strings.TrimSpace(raw[0]) == "" {
			return criteria, fmt.Errorf("nwdaf-event-list must contain at least one value")
		}
		for _, value := range strings.Split(raw[0], ",") {
			event := models.NwdafEvent(strings.TrimSpace(value))
			if event == "" {
				return criteria, fmt.Errorf("nwdaf-event-list contains an empty value")
			}
			criteria.nwdafEvents = append(criteria.nwdafEvents, event)
		}
	}
	if raw, present := values["ml-analytics-info-list"]; present {
		if len(raw) != 1 ||
			json.Unmarshal([]byte(raw[0]), &criteria.mlAnalyticsInfoList) != nil {
			return criteria, fmt.Errorf("ml-analytics-info-list must be a JSON array")
		}
		if len(criteria.mlAnalyticsInfoList) == 0 {
			return criteria, fmt.Errorf("ml-analytics-info-list must contain at least one entry")
		}
		for index, info := range criteria.mlAnalyticsInfoList {
			if info.FLTimeInterval != nil {
				return criteria, fmt.Errorf(
					"ml-analytics-info-list[%d].flTimeInterval matching is unsupported",
					index,
				)
			}
			if err := compatnrf.ValidateMLAnalyticsInfo(info); err != nil {
				return criteria, fmt.Errorf("ml-analytics-info-list[%d]: %w", index, err)
			}
			if info.FLCapabilityType != "" &&
				!compatnrf.IsKnownFLCapability(info.FLCapabilityType) {
				return criteria, fmt.Errorf(
					"ml-analytics-info-list[%d].flCapabilityType is invalid",
					index,
				)
			}
		}
	}
	criteria.internalGroupID = strings.TrimSpace(values.Get("internal-group-identity"))
	var err error
	if criteria.mlModelStorageInd, err = parseTrueIndicator(values, "ml-model-storage-ind"); err != nil {
		return criteria, err
	}
	if criteria.dataStorageInd, err = parseTrueIndicator(values, "data-storage-ind"); err != nil {
		return criteria, err
	}

	target := values.Get("target-nf-type")
	if len(criteria.nwdafEvents) > 0 && target != string(models.NrfNfManagementNfType_NWDAF) {
		return criteria, fmt.Errorf("nwdaf-event-list is only valid for target NWDAF")
	}
	if criteria.mlAnalyticsInfoList != nil && target != string(models.NrfNfManagementNfType_NWDAF) {
		return criteria, fmt.Errorf("ml-analytics-info-list is only valid for target NWDAF")
	}
	if criteria.internalGroupID != "" && target != string(models.NrfNfManagementNfType_UDM) {
		return criteria, fmt.Errorf("internal-group-identity is only valid for target UDM")
	}
	if (criteria.mlModelStorageInd != nil || criteria.dataStorageInd != nil) &&
		target != string(models.NrfNfManagementNfType_ADRF) {
		return criteria, fmt.Errorf("storage indicators are only valid for target ADRF")
	}
	return criteria, nil
}

func parseTrueIndicator(values url.Values, name string) (*bool, error) {
	raw, present := values[name]
	if !present {
		return nil, nil
	}
	if len(raw) != 1 || raw[0] != "true" {
		return nil, fmt.Errorf("%s only supports the standard value true", name)
	}
	value := true
	return &value, nil
}

func requiresCompatibilityDiscovery(
	target string,
	criteria extendedDiscoveryCriteria,
) bool {
	return target == string(models.NrfNfManagementNfType_NWDAF) ||
		target == string(models.NrfNfManagementNfType_ADRF) ||
		criteria.internalGroupID != "" ||
		len(criteria.nwdafEvents) > 0 ||
		criteria.mlAnalyticsInfoList != nil
}

func filterCompatibilityProfiles(
	rawProfiles []map[string]interface{},
	criteria extendedDiscoveryCriteria,
) ([]map[string]interface{}, error) {
	result := make([]map[string]interface{}, 0, len(rawProfiles))
	for index, rawProfile := range rawProfiles {
		encoded, err := json.Marshal(rawProfile)
		if err != nil {
			return nil, fmt.Errorf("encode candidate profile %d: %w", index, err)
		}
		var profile compatnrf.NFProfile
		if err = json.Unmarshal(encoded, &profile); err != nil {
			return nil, fmt.Errorf("decode candidate profile %d: %w", index, err)
		}
		if !matchesExtendedCriteria(profile, criteria) {
			continue
		}
		delete(rawProfile, "_id")
		result = append(result, rawProfile)
	}
	return result, nil
}

func matchesExtendedCriteria(
	profile compatnrf.NFProfile,
	criteria extendedDiscoveryCriteria,
) bool {
	nwdafInfos := profileNwdafInfos(profile)
	if len(criteria.nwdafEvents) > 0 {
		matched := false
		for _, info := range nwdafInfos {
			matched = matched || overlapsNwdafEvents(criteria.nwdafEvents, info.NwdafEvents)
		}
		if !matched {
			return false
		}
	}
	if criteria.mlAnalyticsInfoList != nil {
		profileEntries := make([]compatnrf.MLAnalyticsInfo, 0)
		for _, info := range nwdafInfos {
			profileEntries = append(profileEntries, info.MLAnalyticsList...)
		}
		queryMatched := false
		for _, queryEntry := range criteria.mlAnalyticsInfoList {
			for _, profileEntry := range profileEntries {
				if matchesMLAnalyticsInfo(queryEntry, profileEntry) {
					queryMatched = true
					break
				}
			}
			if queryMatched {
				break
			}
		}
		if !queryMatched {
			return false
		}
	}
	if criteria.internalGroupID != "" &&
		!matchesInternalGroup(profile.NrfNfManagementNfProfile, criteria.internalGroupID) {
		return false
	}
	if criteria.mlModelStorageInd != nil {
		matched := false
		for _, info := range profile.AdrfInfoList {
			matched = matched || info.MLModelStorageInd
		}
		if !matched {
			return false
		}
	}
	if criteria.dataStorageInd != nil {
		matched := false
		for _, info := range profile.AdrfInfoList {
			matched = matched || info.DataStorageInd
		}
		if !matched {
			return false
		}
	}
	return true
}

func profileNwdafInfos(profile compatnrf.NFProfile) []compatnrf.NwdafInfo {
	infos := make([]compatnrf.NwdafInfo, 0, 1+len(profile.NwdafInfoList))
	if profile.NwdafInfo != nil {
		infos = append(infos, *profile.NwdafInfo)
	}
	for _, info := range profile.NwdafInfoList {
		infos = append(infos, info)
	}
	return infos
}

func matchesMLAnalyticsInfo(
	query compatnrf.MLAnalyticsInfo,
	profile compatnrf.MLAnalyticsInfo,
) bool {
	if query.MLAnalyticsIDs != nil &&
		!overlapsNwdafEvents(query.MLAnalyticsIDs, profile.MLAnalyticsIDs) {
		return false
	}
	if query.SNSSAIList != nil && !overlapsJSON(query.SNSSAIList, profile.SNSSAIList) {
		return false
	}
	if query.TrackingAreaList != nil &&
		!overlapsJSON(query.TrackingAreaList, profile.TrackingAreaList) {
		return false
	}
	if query.MLModelInteroperabilityInfo != nil {
		if profile.MLModelInteroperabilityInfo == nil ||
			!overlapsStrings(
				query.MLModelInteroperabilityInfo.VendorList,
				profile.MLModelInteroperabilityInfo.VendorList,
			) {
			return false
		}
	}
	if query.FLCapabilityType != "" &&
		!matchesFLCapability(query.FLCapabilityType, profile.FLCapabilityType) {
		return false
	}
	if query.NFTypeList != nil && !overlapsNFTypes(query.NFTypeList, profile.NFTypeList) {
		return false
	}
	if query.NFSetIDList != nil && !overlapsStrings(query.NFSetIDList, profile.NFSetIDList) {
		return false
	}
	return true
}

func matchesFLCapability(
	query compatnrf.FLCapabilityType,
	profile compatnrf.FLCapabilityType,
) bool {
	switch query {
	case compatnrf.FLCapabilityTypeServer:
		return profile == compatnrf.FLCapabilityTypeServer ||
			profile == compatnrf.FLCapabilityTypeServerAndClient
	case compatnrf.FLCapabilityTypeClient:
		return profile == compatnrf.FLCapabilityTypeClient ||
			profile == compatnrf.FLCapabilityTypeServerAndClient
	case compatnrf.FLCapabilityTypeServerAndClient:
		return profile == compatnrf.FLCapabilityTypeServerAndClient
	default:
		return false
	}
}

func overlapsNwdafEvents(left, right []models.NwdafEvent) bool {
	for _, leftValue := range left {
		for _, rightValue := range right {
			if leftValue == rightValue {
				return true
			}
		}
	}
	return false
}

func overlapsNFTypes(left, right []models.NrfNfManagementNfType) bool {
	for _, leftValue := range left {
		for _, rightValue := range right {
			if leftValue == rightValue {
				return true
			}
		}
	}
	return false
}

func overlapsStrings(left, right []string) bool {
	for _, leftValue := range left {
		for _, rightValue := range right {
			if leftValue == rightValue {
				return true
			}
		}
	}
	return false
}

func overlapsJSON[T any](left, right []T) bool {
	for _, leftValue := range left {
		leftJSON, err := json.Marshal(leftValue)
		if err != nil {
			continue
		}
		for _, rightValue := range right {
			rightJSON, err := json.Marshal(rightValue)
			if err != nil {
				continue
			}
			if string(leftJSON) == string(rightJSON) {
				return true
			}
		}
	}
	return false
}

func matchesInternalGroup(
	profile models.NrfNfManagementNfProfile,
	identity string,
) bool {
	infos := make([]models.UdmInfo, 0, 1+len(profile.UdmInfoList))
	if profile.UdmInfo != nil {
		infos = append(infos, *profile.UdmInfo)
	}
	for _, info := range profile.UdmInfoList {
		infos = append(infos, info)
	}
	for _, info := range infos {
		for _, valueRange := range info.InternalGroupIdentifiersRanges {
			if valueRange.Start != "" && valueRange.End != "" &&
				identity >= valueRange.Start && identity <= valueRange.End {
				return true
			}
			if valueRange.Pattern != "" {
				expression, err := regexp.Compile(valueRange.Pattern)
				if err == nil && expression.MatchString(identity) {
					return true
				}
			}
		}
	}
	return false
}
