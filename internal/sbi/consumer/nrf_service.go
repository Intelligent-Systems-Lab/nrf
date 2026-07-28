package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"

	compatnrf "github.com/free5gc/nrf/internal/compat/nrf"
	"github.com/free5gc/nrf/internal/logger"
	"github.com/free5gc/openapi"
	"github.com/free5gc/openapi/models"
	"github.com/free5gc/openapi/nrf/NFManagement"
	sbi_metrics "github.com/free5gc/util/metrics/sbi"
)

type nnrfService struct {
	consumer *Consumer

	nfMngmntMu sync.RWMutex

	nfMngmntConfigs map[string]*NFManagement.Configuration
}

func (s *nnrfService) getNFManagementConfig(uri string) *NFManagement.Configuration {
	if uri == "" {
		return nil
	}
	s.nfMngmntMu.RLock()
	configuration, ok := s.nfMngmntConfigs[uri]
	if ok {
		defer s.nfMngmntMu.RUnlock()
		return configuration
	}

	s.nfMngmntMu.RUnlock()
	s.nfMngmntMu.Lock()
	defer s.nfMngmntMu.Unlock()
	if configuration, ok = s.nfMngmntConfigs[uri]; ok {
		return configuration
	}
	configuration = NFManagement.NewConfiguration()
	configuration.SetBasePath(uri)
	configuration.SetMetrics(sbi_metrics.SbiMetricHook)
	s.nfMngmntConfigs[uri] = configuration
	return configuration
}

type nfStatusNotification struct {
	Event         models.NotificationEventType `json:"event"`
	NfInstanceURI string                       `json:"nfInstanceUri"`
	NfProfile     *compatnrf.NFProfile         `json:"nfProfile,omitempty"`
}

func (s *nnrfService) SendNFStatusNotify(
	ctx context.Context,
	notification_event models.NotificationEventType,
	nfInstanceUri string,
	callbackURL string,
	nfProfile *compatnrf.NFProfile,
) *models.ProblemDetails {
	logger.ConsumerLog.Infoln("SendNFStatusNotify")

	configuration := s.getNFManagementConfig(callbackURL)
	if configuration == nil {
		return &models.ProblemDetails{
			Status: http.StatusInternalServerError,
			Cause:  "NEW_CLIENT_ERROR",
			Detail: fmt.Sprintf("Can't Get/New Client for url for [%+v]", callbackURL),
		}
	}
	s.nfMngmntMu.RLock()
	defer s.nfMngmntMu.RUnlock()

	notificationData := nfStatusNotification{
		Event:         notification_event,
		NfInstanceURI: nfInstanceUri,
		NfProfile:     nfProfile,
	}
	headers := map[string]string{
		"Accept":       "application/json, application/problem+json",
		"Content-Type": "application/json",
	}
	request, err := openapi.PrepareRequest(
		ctx,
		configuration,
		callbackURL,
		http.MethodPost,
		notificationData,
		headers,
		url.Values{},
		url.Values{},
		"",
		"",
		nil,
	)
	if err != nil {
		return &models.ProblemDetails{
			Status: http.StatusInternalServerError,
			Cause:  "NOTIFICATION_ERROR",
			Detail: err.Error(),
		}
	}
	response, err := openapi.CallAPI(configuration, request)
	if err != nil || response == nil {
		if err == nil {
			err = fmt.Errorf("notification response is unavailable")
		}
		logger.NfmLog.Infof("Notify fail: %v", err)
		return &models.ProblemDetails{
			Status: http.StatusInternalServerError,
			Cause:  "NOTIFICATION_ERROR",
			Detail: err.Error(),
		}
	}
	defer func() {
		if closeErr := response.Body.Close(); closeErr != nil {
			logger.NfmLog.Warnf("Close notification response body: %v", closeErr)
		}
	}()
	body, readErr := io.ReadAll(response.Body)
	if readErr != nil {
		return &models.ProblemDetails{
			Status: http.StatusInternalServerError,
			Cause:  "NOTIFICATION_ERROR",
			Detail: readErr.Error(),
		}
	}
	if response.StatusCode == http.StatusNoContent {
		return nil
	}
	var peerProblem models.ProblemDetails
	if len(body) > 0 && json.Unmarshal(body, &peerProblem) == nil {
		if peerProblem.Status == 0 {
			peerProblem.Status = int32(response.StatusCode)
		}
		return &peerProblem
	}
	return &models.ProblemDetails{
		Status: int32(response.StatusCode),
		Cause:  "NOTIFICATION_ERROR",
		Detail: fmt.Sprintf("notification peer returned HTTP %d", response.StatusCode),
	}
}
