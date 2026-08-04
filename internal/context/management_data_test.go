package context

import (
	"testing"

	"github.com/free5gc/nrf/pkg/factory"
	"github.com/free5gc/openapi/models"
)

func TestNnrfNFManagementDataModelPreservesUDMInternalGroupRanges(t *testing.T) {
	previousConfig := factory.NrfConfig
	factory.NrfConfig = &factory.Config{
		Configuration: &factory.Configuration{Sbi: &factory.Sbi{}},
	}
	t.Cleanup(func() { factory.NrfConfig = previousConfig })
	source := &models.NrfNfManagementNfProfile{
		NfInstanceId: "10000000-0000-4000-8000-000000000013",
		NfType:       models.NrfNfManagementNfType_UDM,
		NfStatus:     models.NrfNfManagementNfStatus_REGISTERED,
		PlmnList:     []models.PlmnId{{Mcc: "466", Mnc: "92"}},
		UdmInfo: &models.UdmInfo{
			InternalGroupIdentifiersRanges: []models.InternalGroupIdRange{
				{
					Start: "00000001-466-92-01",
					End:   "00000001-466-92-01",
				},
			},
		},
	}
	var persisted models.NrfNfManagementNfProfile
	if err := NnrfNFManagementDataModel(&persisted, source); err != nil {
		t.Fatalf("NnrfNFManagementDataModel() error = %v", err)
	}
	if persisted.UdmInfo == nil || len(persisted.UdmInfo.InternalGroupIdentifiersRanges) != 1 {
		t.Fatalf("persisted UDM info = %+v", persisted.UdmInfo)
	}
	got := persisted.UdmInfo.InternalGroupIdentifiersRanges[0]
	if got.Start != "00000001-466-92-01" || got.End != "00000001-466-92-01" {
		t.Fatalf("persisted internal group range = %+v", got)
	}
}
