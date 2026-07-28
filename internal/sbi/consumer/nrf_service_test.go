package consumer

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	compatnrf "github.com/free5gc/nrf/internal/compat/nrf"
	"github.com/free5gc/openapi"
	"github.com/free5gc/openapi/models"
	"github.com/free5gc/openapi/nrf/NFManagement"
)

func TestSendNFStatusNotifyPreservesRelease18Profile(t *testing.T) {
	t.Parallel()

	var received map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		if request.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", request.Method)
		}
		if authorization := request.Header.Get("Authorization"); authorization != "Bearer token" {
			t.Errorf("Authorization = %q, want Bearer token", authorization)
		}
		if contentType := request.Header.Get("Content-Type"); contentType != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", contentType)
		}
		if err := json.NewDecoder(request.Body).Decode(&received); err != nil {
			t.Errorf("decode notification: %v", err)
		}
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	configuration := NFManagement.NewConfiguration()
	configuration.SetBasePath(server.URL)
	configuration.SetHTTPClient(server.Client())
	service := &nnrfService{
		nfMngmntConfigs: map[string]*NFManagement.Configuration{
			server.URL: configuration,
		},
	}
	profile := &compatnrf.NFProfile{
		NrfNfManagementNfProfile: models.NrfNfManagementNfProfile{
			NfInstanceId: "11111111-1111-4111-8111-111111111111",
			NfType:       models.NrfNfManagementNfType_NWDAF,
			NfStatus:     models.NrfNfManagementNfStatus_REGISTERED,
		},
		NwdafInfo: &compatnrf.NwdafInfo{
			MLAnalyticsList: []compatnrf.MLAnalyticsInfo{{
				MLAnalyticsIDs:   []models.NwdafEvent{models.NwdafEvent_UE_COMMUNICATION},
				FLCapabilityType: compatnrf.FLCapabilityTypeClient,
				TrackingAreaList: []models.Tai{{
					PlmnId: &models.PlmnId{Mcc: "001", Mnc: "01"},
					Tac:    "000001",
				}},
			}},
		},
	}
	ctx := context.WithValue(context.Background(), openapi.ContextAccessToken, "token")
	if problem := service.SendNFStatusNotify(
		ctx,
		models.NotificationEventType_PROFILE_CHANGED,
		server.URL+"/nnrf-nfm/v1/nf-instances/"+profile.NfInstanceId,
		server.URL,
		profile,
	); problem != nil {
		t.Fatalf("SendNFStatusNotify() problem = %+v", problem)
	}

	encoded, err := json.Marshal(received["nfProfile"])
	if err != nil {
		t.Fatalf("encode received profile: %v", err)
	}
	var roundTrip compatnrf.NFProfile
	if err = json.Unmarshal(encoded, &roundTrip); err != nil {
		t.Fatalf("decode received profile: %v", err)
	}
	entry := roundTrip.NwdafInfo.MLAnalyticsList[0]
	if entry.FLCapabilityType != compatnrf.FLCapabilityTypeClient {
		t.Fatalf("flCapabilityType = %q, want FL_CLIENT", entry.FLCapabilityType)
	}
	if len(entry.TrackingAreaList) != 1 || entry.TrackingAreaList[0].Tac != "000001" {
		t.Fatalf("trackingAreaList = %+v, want TAC 000001", entry.TrackingAreaList)
	}
}
