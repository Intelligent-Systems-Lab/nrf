package processor

import (
	"net/url"
	"testing"

	compatnrf "github.com/free5gc/nrf/internal/compat/nrf"
	"github.com/free5gc/openapi/models"
)

func TestMatchesMLAnalyticsInfoRequiresSameProfileEntry(t *testing.T) {
	t.Parallel()

	taiA := models.Tai{
		PlmnId: &models.PlmnId{Mcc: "466", Mnc: "92"},
		Tac:    "000001",
	}
	profile := compatnrf.NFProfile{
		NwdafInfo: &compatnrf.NwdafInfo{
			MLAnalyticsList: []compatnrf.MLAnalyticsInfo{
				{MLAnalyticsIDs: []models.NwdafEvent{models.NwdafEvent_UE_COMMUNICATION}},
				{
					MLAnalyticsIDs:   []models.NwdafEvent{"ABNORMAL_BEHAVIOUR"},
					TrackingAreaList: []models.Tai{taiA},
				},
			},
		},
	}
	criteria := extendedDiscoveryCriteria{
		mlAnalyticsInfoList: []compatnrf.MLAnalyticsInfo{{
			MLAnalyticsIDs:   []models.NwdafEvent{models.NwdafEvent_UE_COMMUNICATION},
			TrackingAreaList: []models.Tai{taiA},
		}},
	}
	if matchesExtendedCriteria(profile, criteria) {
		t.Fatal("split attributes across two profile entries unexpectedly matched")
	}
}

func TestMatchesMLAnalyticsInfoSupportsOverlapAndCombinedCapability(t *testing.T) {
	t.Parallel()

	taiA := models.Tai{PlmnId: &models.PlmnId{Mcc: "466", Mnc: "92"}, Tac: "000001"}
	taiB := models.Tai{PlmnId: &models.PlmnId{Mcc: "466", Mnc: "92"}, Tac: "000002"}
	profile := compatnrf.NFProfile{
		NwdafInfo: &compatnrf.NwdafInfo{
			MLAnalyticsList: []compatnrf.MLAnalyticsInfo{{
				MLAnalyticsIDs:   []models.NwdafEvent{models.NwdafEvent_UE_COMMUNICATION},
				TrackingAreaList: []models.Tai{taiA},
				FLCapabilityType: compatnrf.FLCapabilityTypeServerAndClient,
				NFTypeList:       []models.NrfNfManagementNfType{models.NrfNfManagementNfType_UPF},
			}},
		},
	}
	criteria := extendedDiscoveryCriteria{
		mlAnalyticsInfoList: []compatnrf.MLAnalyticsInfo{{
			MLAnalyticsIDs:   []models.NwdafEvent{models.NwdafEvent_UE_COMMUNICATION},
			TrackingAreaList: []models.Tai{taiA, taiB},
			FLCapabilityType: compatnrf.FLCapabilityTypeClient,
			NFTypeList:       []models.NrfNfManagementNfType{models.NrfNfManagementNfType_UPF},
		}},
	}
	if !matchesExtendedCriteria(profile, criteria) {
		t.Fatal("TAI overlap and combined FL capability should match FL client query")
	}
}

func TestMatchesADRFStorageCapabilities(t *testing.T) {
	t.Parallel()

	profile := compatnrf.NFProfile{
		AdrfInfoList: map[string]compatnrf.AdrfInfo{
			"region-a": {DataStorageInd: true},
		},
	}
	required := true
	if !matchesExtendedCriteria(profile, extendedDiscoveryCriteria{
		dataStorageInd: &required,
	}) {
		t.Fatal("ADRF data storage capability did not match")
	}
	if matchesExtendedCriteria(profile, extendedDiscoveryCriteria{
		mlModelStorageInd: &required,
	}) {
		t.Fatal("ADRF model storage capability unexpectedly matched")
	}
}

func TestParseExtendedDiscoveryCriteriaRejectsUnsupportedShapes(t *testing.T) {
	t.Parallel()

	for name, values := range map[string]url.Values{
		"empty ML list": {
			"target-nf-type":         {"NWDAF"},
			"ml-analytics-info-list": {"[]"},
		},
		"unsupported time matching": {
			"target-nf-type": {"NWDAF"},
			"ml-analytics-info-list": {`[{
				"mlAnalyticsIds":["UE_COMMUNICATION"],
				"flTimeInterval":{
					"startTime":"2026-07-28T00:00:00Z",
					"stopTime":"2026-07-28T01:00:00Z"
				}
			}]`},
		},
		"false storage indicator": {
			"target-nf-type":   {"ADRF"},
			"data-storage-ind": {"false"},
		},
	} {
		name, values := name, values
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := parseExtendedDiscoveryCriteria(values); err == nil {
				t.Fatal("parseExtendedDiscoveryCriteria() error = nil")
			}
		})
	}
}
