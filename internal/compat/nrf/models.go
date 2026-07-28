// Package nrf contains Release 18 NF profile fields missing from the pinned
// free5GC OpenAPI module.
package nrf

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"

	"github.com/free5gc/openapi/models"
)

type FLCapabilityType string

const (
	FLCapabilityTypeServer          FLCapabilityType = "FL_SERVER"
	FLCapabilityTypeClient          FLCapabilityType = "FL_CLIENT"
	FLCapabilityTypeServerAndClient FLCapabilityType = "FL_SERVER_AND_CLIENT"
)

type MLModelInteroperabilityInfo struct {
	VendorList []string `json:"vendorList,omitempty" bson:"vendorList,omitempty"`
}

//nolint:lll // JSON and BSON tags intentionally repeat the standard wire property names.
type MLAnalyticsInfo struct {
	MLAnalyticsIDs              []models.NwdafEvent            `json:"mlAnalyticsIds,omitempty" bson:"mlAnalyticsIds,omitempty"`
	SNSSAIList                  []models.Snssai                `json:"snssaiList,omitempty" bson:"snssaiList,omitempty"`
	TrackingAreaList            []models.Tai                   `json:"trackingAreaList,omitempty" bson:"trackingAreaList,omitempty"`
	MLModelInteroperabilityInfo *MLModelInteroperabilityInfo   `json:"mlModelInterInfo,omitempty" bson:"mlModelInterInfo,omitempty"`
	FLCapabilityType            FLCapabilityType               `json:"flCapabilityType,omitempty" bson:"flCapabilityType,omitempty"`
	FLTimeInterval              *models.TimeWindow             `json:"flTimeInterval,omitempty" bson:"flTimeInterval,omitempty"`
	NFTypeList                  []models.NrfNfManagementNfType `json:"nfTypeList,omitempty" bson:"nfTypeList,omitempty"`
	NFSetIDList                 []string                       `json:"nfSetIdList,omitempty" bson:"nfSetIdList,omitempty"`
}

type NwdafInfo struct {
	models.NwdafInfo
	MLAnalyticsList []MLAnalyticsInfo
}

type AdrfInfo struct {
	MLModelStorageInd bool `json:"mlModelStorageInd,omitempty" bson:"mlModelStorageInd,omitempty"`
	DataStorageInd    bool `json:"dataStorageInd,omitempty" bson:"dataStorageInd,omitempty"`
}

type NFProfile struct {
	models.NrfNfManagementNfProfile
	NwdafInfo     *NwdafInfo
	NwdafInfoList map[string]NwdafInfo
	AdrfInfoList  map[string]AdrfInfo
}

var vendorIDPattern = regexp.MustCompile(`^[0-9]{6}$`)

func ValidateMLAnalyticsInfo(value MLAnalyticsInfo) error {
	for name, item := range map[string]struct {
		length  int
		present bool
	}{
		"mlAnalyticsIds":   {len(value.MLAnalyticsIDs), value.MLAnalyticsIDs != nil},
		"snssaiList":       {len(value.SNSSAIList), value.SNSSAIList != nil},
		"trackingAreaList": {len(value.TrackingAreaList), value.TrackingAreaList != nil},
		"nfTypeList":       {len(value.NFTypeList), value.NFTypeList != nil},
		"nfSetIdList":      {len(value.NFSetIDList), value.NFSetIDList != nil},
	} {
		if item.present && item.length == 0 {
			return fmt.Errorf("%s must contain at least one item when present", name)
		}
	}
	if len(value.MLAnalyticsIDs) == 0 {
		return errors.New("mlAnalyticsIds must contain at least one item")
	}
	if value.MLModelInteroperabilityInfo != nil {
		vendors := value.MLModelInteroperabilityInfo.VendorList
		if vendors != nil && len(vendors) == 0 {
			return errors.New("mlModelInterInfo.vendorList must contain at least one item when present")
		}
		for index, vendor := range vendors {
			if !vendorIDPattern.MatchString(vendor) {
				return fmt.Errorf(
					"mlModelInterInfo.vendorList[%d] must be a six-digit Vendor ID",
					index,
				)
			}
		}
	}
	if value.FLTimeInterval != nil {
		if value.FLTimeInterval.StartTime == nil || value.FLTimeInterval.StopTime == nil {
			return errors.New("flTimeInterval requires startTime and stopTime")
		}
		if value.FLTimeInterval.StopTime.Before(*value.FLTimeInterval.StartTime) {
			return errors.New("flTimeInterval stopTime must not precede startTime")
		}
	}
	if value.FLCapabilityType == "" &&
		(value.FLTimeInterval != nil || value.NFTypeList != nil || value.NFSetIDList != nil) {
		return errors.New("flTimeInterval, nfTypeList and nfSetIdList require flCapabilityType")
	}
	return nil
}

func IsKnownFLCapability(value FLCapabilityType) bool {
	switch value {
	case FLCapabilityTypeServer, FLCapabilityTypeClient, FLCapabilityTypeServerAndClient:
		return true
	default:
		return false
	}
}

func (value NwdafInfo) MarshalJSON() ([]byte, error) {
	encoded, err := json.Marshal(value.NwdafInfo)
	if err != nil {
		return nil, err
	}
	var object map[string]json.RawMessage
	if err = json.Unmarshal(encoded, &object); err != nil {
		return nil, err
	}
	if value.MLAnalyticsList != nil {
		encodedList, marshalErr := json.Marshal(value.MLAnalyticsList)
		if marshalErr != nil {
			return nil, marshalErr
		}
		object["mlAnalyticsList"] = encodedList
	}
	return json.Marshal(object)
}

func (value *NwdafInfo) UnmarshalJSON(data []byte) error {
	var generated models.NwdafInfo
	if err := json.Unmarshal(data, &generated); err != nil {
		return err
	}
	var extension struct {
		MLAnalyticsList []MLAnalyticsInfo `json:"mlAnalyticsList"`
	}
	if err := json.Unmarshal(data, &extension); err != nil {
		return err
	}
	value.NwdafInfo = generated
	value.NwdafInfo.MlAnalyticsList = nil
	value.MLAnalyticsList = extension.MLAnalyticsList
	return nil
}

//nolint:gocritic // Value receiver preserves json.Marshaler on profile values.
func (value NFProfile) MarshalJSON() ([]byte, error) {
	encoded, err := json.Marshal(value.NrfNfManagementNfProfile)
	if err != nil {
		return nil, err
	}
	var object map[string]json.RawMessage
	if err = json.Unmarshal(encoded, &object); err != nil {
		return nil, err
	}
	for name, field := range map[string]any{
		"nwdafInfo":     value.NwdafInfo,
		"nwdafInfoList": value.NwdafInfoList,
		"adrfInfoList":  value.AdrfInfoList,
	} {
		if field == nil {
			continue
		}
		encodedField, marshalErr := json.Marshal(field)
		if marshalErr != nil {
			return nil, marshalErr
		}
		object[name] = encodedField
	}
	return json.Marshal(object)
}

func (value *NFProfile) UnmarshalJSON(data []byte) error {
	var generated models.NrfNfManagementNfProfile
	if err := json.Unmarshal(data, &generated); err != nil {
		return err
	}
	var extension struct {
		NwdafInfo     *NwdafInfo           `json:"nwdafInfo"`
		NwdafInfoList map[string]NwdafInfo `json:"nwdafInfoList"`
		AdrfInfoList  map[string]AdrfInfo  `json:"adrfInfoList"`
	}
	if err := json.Unmarshal(data, &extension); err != nil {
		return err
	}
	value.NrfNfManagementNfProfile = generated
	value.NrfNfManagementNfProfile.NwdafInfo = nil
	value.NrfNfManagementNfProfile.NwdafInfoList = nil
	value.NwdafInfo = extension.NwdafInfo
	value.NwdafInfoList = extension.NwdafInfoList
	value.AdrfInfoList = extension.AdrfInfoList
	return nil
}
