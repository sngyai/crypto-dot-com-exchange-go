package cdcexchange_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cdcexchange "github.com/sngyai/go-cryptocom"
	cdcerrors "github.com/sngyai/go-cryptocom/errors"
	"github.com/sngyai/go-cryptocom/internal/api"
	"github.com/sngyai/go-cryptocom/internal/auth"
	id_mocks "github.com/sngyai/go-cryptocom/internal/mocks/id"
	signature_mocks "github.com/sngyai/go-cryptocom/internal/mocks/signature"
)

func TestClient_CancelAllOrders_Error(t *testing.T) {
	const (
		apiKey         = "some api key"
		secretKey      = "some secret key"
		id             = int64(1234)
		instrumentName = "some instrument name"
	)
	testErr := errors.New("some error")

	type args struct {
		instrumentName string
	}
	tests := []struct {
		name string
		args
		client       http.Client
		signatureErr error
		responseErr  error
		expectedErr  error
	}{
		{
			name: "returns error when instrument name is empty",
			args: args{
				instrumentName: "",
			},
			expectedErr: cdcerrors.InvalidParameterError{
				Parameter: "instrumentName",
				Reason:    "cannot be empty",
			},
		},
		{
			name: "returns error given error generating signature",
			args: args{
				instrumentName: instrumentName,
			},
			signatureErr: testErr,
			expectedErr:  testErr,
		},
		{
			name: "returns error given error making request",
			args: args{
				instrumentName: instrumentName,
			},
			client: http.Client{
				Transport: roundTripper{
					err: testErr,
				},
			},
			expectedErr: testErr,
		},
		{
			name: "returns error given error response",
			args: args{
				instrumentName: instrumentName,
			},
			client: http.Client{
				Transport: roundTripper{
					statusCode: http.StatusTeapot,
					response: api.BaseResponse{
						Code: "10003",
					},
				},
			},
			expectedErr: cdcerrors.ResponseError{
				Code:           10003,
				HTTPStatusCode: http.StatusTeapot,
				Err:            cdcerrors.ErrIllegalIP,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl, ctx := gomock.WithContext(context.Background(), t)
			t.Cleanup(ctrl.Finish)

			var (
				idGenerator        = id_mocks.NewMockIDGenerator(ctrl)
				signatureGenerator = signature_mocks.NewMockSignatureGenerator(ctrl)
				now                = time.Now()
				clock              = clockwork.NewFakeClockAt(now)
			)

			client, err := cdcexchange.New(apiKey, secretKey,
				cdcexchange.WithIDGenerator(idGenerator),
				cdcexchange.WithClock(clock),
				cdcexchange.WithHTTPClient(&tt.client),
				cdcexchange.WithSignatureGenerator(signatureGenerator),
			)
			require.NoError(t, err)

			if tt.instrumentName != "" {
				idGenerator.EXPECT().Generate().Return(id)
				signatureGenerator.EXPECT().GenerateSignature(auth.SignatureRequest{
					APIKey:    apiKey,
					SecretKey: secretKey,
					ID:        id,
					Method:    cdcexchange.MethodCancelAllOrders,
					Timestamp: now.UnixMilli(),
					Params: map[string]interface{}{
						"instrument_name": instrumentName,
					},
				}).Return("signature", tt.signatureErr)
			}

			err = client.CancelAllOrders(ctx, tt.instrumentName)
			require.Error(t, err)

			assert.True(t, errors.Is(err, tt.expectedErr))

			var expectedResponseError cdcerrors.ResponseError
			if errors.As(tt.expectedErr, &expectedResponseError) {
				var responseError cdcerrors.ResponseError
				require.True(t, errors.As(err, &responseError))

				assert.Equal(t, expectedResponseError.Code, responseError.Code)
				assert.Equal(t, expectedResponseError.HTTPStatusCode, responseError.HTTPStatusCode)
				assert.Equal(t, expectedResponseError.Err, responseError.Err)

				assert.True(t, errors.Is(err, expectedResponseError.Err))
			}
		})
	}
}

func TestClient_CancelAllOrders_Success(t *testing.T) {
	const (
		apiKey    = "some api key"
		secretKey = "some secret key"
		id        = int64(1234)
		signature = "some signature"

		instrumentName = "some instrument name"
	)
	now := time.Now()

	type args struct {
		instrumentName string
	}
	tests := []struct {
		name        string
		handlerFunc func(w http.ResponseWriter, r *http.Request)
		args
		expectedParams map[string]interface{}
	}{
		{
			name: "successfully cancels an order",
			args: args{
				instrumentName: instrumentName,
			},
			handlerFunc: func(w http.ResponseWriter, r *http.Request) {
				assert.Contains(t, r.URL.Path, cdcexchange.MethodCancelAllOrders)
				t.Cleanup(func() { require.NoError(t, r.Body.Close()) })

				var body api.Request
				require.NoError(t, json.NewDecoder(r.Body).Decode(&body))

				assert.Equal(t, cdcexchange.MethodCancelAllOrders, body.Method)
				assert.Equal(t, id, body.ID)
				assert.Equal(t, apiKey, body.APIKey)
				assert.Equal(t, now.UnixMilli(), body.Nonce)
				assert.Equal(t, signature, body.Signature)
				assert.Equal(t, instrumentName, body.Params["instrument_name"])

				res := cdcexchange.CancelAllOrdersResponse{
					BaseResponse: api.BaseResponse{},
				}

				require.NoError(t, json.NewEncoder(w).Encode(res))
			},
			expectedParams: map[string]interface{}{
				"instrument_name": instrumentName,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl, ctx := gomock.WithContext(context.Background(), t)
			t.Cleanup(ctrl.Finish)

			var (
				signatureGenerator = signature_mocks.NewMockSignatureGenerator(ctrl)
				idGenerator        = id_mocks.NewMockIDGenerator(ctrl)
				clock              = clockwork.NewFakeClockAt(now)
			)

			s := httptest.NewServer(http.HandlerFunc(tt.handlerFunc))
			t.Cleanup(s.Close)

			client, err := cdcexchange.New(apiKey, secretKey,
				cdcexchange.WithIDGenerator(idGenerator),
				cdcexchange.WithClock(clock),
				cdcexchange.WithHTTPClient(s.Client()),
				cdcexchange.WithBaseURL(fmt.Sprintf("%s/", s.URL)),
				cdcexchange.WithSignatureGenerator(signatureGenerator),
			)
			require.NoError(t, err)

			idGenerator.EXPECT().Generate().Return(id)
			signatureGenerator.EXPECT().GenerateSignature(auth.SignatureRequest{
				APIKey:    apiKey,
				SecretKey: secretKey,
				ID:        id,
				Method:    cdcexchange.MethodCancelAllOrders,
				Timestamp: now.UnixMilli(),
				Params:    tt.expectedParams,
			}).Return(signature, nil)

			err = client.CancelAllOrders(ctx, tt.instrumentName)
			require.NoError(t, err)
		})
	}
}
