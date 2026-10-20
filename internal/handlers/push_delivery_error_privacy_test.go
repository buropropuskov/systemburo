package handlers_test

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type pushPrivateErrorClient struct{}

func (pushPrivateErrorClient) Do(req *http.Request) (*http.Response, error) {
	if req.Body != nil {
		_ = req.Body.Close()
	}
	return nil, &url.Error{Op: "Post", URL: req.URL.String(), Err: errors.New("proxy password=synthetic-private-password")}
}

func TestPushDelivery_ErrorDetailsNotStored(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	userID := seedPushUser(t, db, td, "push_private_error")
	seedSubscription(t, db, userID, "http://127.0.0.1/synthetic-endpoint-token")
	privateKey, publicKey, err := webpush.GenerateVAPIDKeys()
	require.NoError(t, err)
	svc := services.NewPushService(db, publicKey, privateKey, testPushSubject, services.WithPushSyncSend(), services.WithPushLocalEndpoints(), services.WithPushHTTPClient(pushPrivateErrorClient{}))
	svc.Send(context.Background(), userID, services.PushPayload{Title: "Synthetic", Message: "Synthetic", Type: "application_created"})
	var subscription models.PushSubscription
	require.NoError(t, db.Where("user_id = ?", userID).First(&subscription).Error)
	assert.Equal(t, 1, subscription.FailedCount)
	require.NotNil(t, subscription.LastError)
	assert.NotContains(t, *subscription.LastError, "synthetic-private-password")
	assert.NotContains(t, *subscription.LastError, "synthetic-endpoint-token")
	assert.Contains(t, *subscription.LastError, "ошибка соединения")
}
