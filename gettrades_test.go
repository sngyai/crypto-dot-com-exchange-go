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
	cdctime "github.com/sngyai/go-cryptocom/internal/time"
)

func TestClient_GetTrades_Error(t *testing.T) {
	const (
		apiKey    = "some api key"
		secretKey = "some secret key"
		id        = int64(1234)
	)
	testErr := errors.New("some error")

	type args struct {
		req cdcexchange.GetTradesRequest
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
			name: "returns error when page size is less than 0",
			args: args{
				req: cdcexchange.GetTradesRequest{
					PageSize: -1,
				},
			},
			expectedErr: cdcerrors.InvalidParameterError{
				Parameter: "req.Limit",
				Reason:    "cannot be less than 0",
			},
		},
		{
			name: "returns error when page size is greater than 200",
			args: args{
				req: cdcexchange.GetTradesRequest{
					PageSize: 201,
				},
			},
			expectedErr: cdcerrors.InvalidParameterError{
				Parameter: "req.Limit",
				Reason:    "cannot be greater than 200",
			},
		},
		{
			name:         "returns error given error generating signature",
			signatureErr: testErr,
			expectedErr:  testErr,
		},
		{
			name: "returns error given error making request",
			client: http.Client{
				Transport: roundTripper{
					err: testErr,
				},
			},
			expectedErr: testErr,
		},
		{
			name: "returns error given error response",
			client: http.Client{
				Transport: roundTripper{
					statusCode: http.StatusTeapot,
					response: api.BaseResponse{
						Code: "10003",
					},
				},
			},
			responseErr: nil,
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

			if tt.req.PageSize >= 0 && tt.req.PageSize < 200 {
				idGenerator.EXPECT().Generate().Return(id)
				signatureGenerator.EXPECT().GenerateSignature(auth.SignatureRequest{
					APIKey:    apiKey,
					SecretKey: secretKey,
					ID:        id,
					Method:    cdcexchange.MethodGetTrades,
					Timestamp: now.UnixMilli(),
					Params:    map[string]interface{}{"page": 0},
				}).Return("signature", tt.signatureErr)
			}

			res, err := client.GetTrades(ctx, tt.req)
			require.Error(t, err)

			assert.Empty(t, res)

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

func TestClient_GetTrades_Success(t *testing.T) {
	const (
		apiKey    = "some api key"
		secretKey = "some secret key"
		id        = int64(1234)
		signature = "some signature"

		instrument = "some instrument"
		clientOID  = "some Client oid"
	)
	now := time.Now().Round(time.Second)

	type args struct {
		req cdcexchange.GetTradesRequest
	}
	tests := []struct {
		name        string
		handlerFunc func(w http.ResponseWriter, r *http.Request)
		args
		expectedParams map[string]interface{}
		expectedResult []cdcexchange.Trade
	}{
		{
			name: "successfully gets all trades for an instrument",
			args: args{
				req: cdcexchange.GetTradesRequest{
					InstrumentName: instrument,
					PageSize:       100,
					Page:           1,
				},
			},
			handlerFunc: func(w http.ResponseWriter, r *http.Request) {
				assert.Contains(t, r.URL.Path, cdcexchange.MethodGetTrades)
				t.Cleanup(func() { require.NoError(t, r.Body.Close()) })

				var body api.Request
				require.NoError(t, json.NewDecoder(r.Body).Decode(&body))

				assert.Equal(t, cdcexchange.MethodGetTrades, body.Method)
				assert.Equal(t, id, body.ID)
				assert.Equal(t, apiKey, body.APIKey)
				assert.Equal(t, now.UnixMilli(), body.Nonce)
				assert.Equal(t, signature, body.Signature)
				assert.Equal(t, instrument, body.Params["instrument_name"])
				assert.Equal(t, float64(100), body.Params["page_size"])
				assert.Equal(t, float64(1), body.Params["page"])

				res := fmt.Sprintf(`{
							"id": 0,
							"method":"",
							"code":0,
							"result":{
								"trade_list":[
									{
										"side": "SELL",
										"instrument_name": "ETH_CRO",
										"fee": 0.014,
										"trade_id": "367107655537806900",
										"create_time": %d,
										"traded_price": 7,
										"traded_quantity": 1,
										"fee_currency": "CRO",
										"order_id": "367107623521528450"
								   }
								]
							}
						}`, now.UnixMilli())

				_, err := w.Write([]byte(res))
				require.NoError(t, err)
			},
			expectedParams: map[string]interface{}{
				"instrument_name": instrument,
				"page_size":       100,
				"page":            1,
			},
			expectedResult: []cdcexchange.Trade{
				{
					Side:           cdcexchange.OrderSideSell,
					InstrumentName: "ETH_CRO",
					Fee:            0.014,
					TradeID:        "367107655537806900",
					CreateTime:     cdctime.Time(now),
					TradedPrice:    7,
					TradedQuantity: 1,
					FeeCurrency:    "CRO",
					OrderID:        "367107623521528450",
				},
			},
		},
		{
			name: "successfully gets all trades for all instruments",
			args: args{
				req: cdcexchange.GetTradesRequest{},
			},
			handlerFunc: func(w http.ResponseWriter, r *http.Request) {
				assert.Contains(t, r.URL.Path, cdcexchange.MethodGetTrades)
				t.Cleanup(func() { require.NoError(t, r.Body.Close()) })

				var body api.Request
				require.NoError(t, json.NewDecoder(r.Body).Decode(&body))

				assert.Equal(t, cdcexchange.MethodGetTrades, body.Method)
				assert.Equal(t, id, body.ID)
				assert.Equal(t, apiKey, body.APIKey)
				assert.Equal(t, now.UnixMilli(), body.Nonce)
				assert.Equal(t, signature, body.Signature)
				assert.Equal(t, float64(0), body.Params["page"])

				res := fmt.Sprintf(`{
							"id": 0,
							"method":"",
							"code":0,
							"result":{
								"trade_list":[
									{
										"side": "SELL",
										"instrument_name": "ETH_CRO",
										"fee": 0.014,
										"trade_id": "367107655537806900",
										"create_time": %d,
										"traded_price": 7,
										"traded_quantity": 1,
										"fee_currency": "CRO",
										"order_id": "367107623521528450"
								   }
								]
							}
						}`, now.UnixMilli())

				_, err := w.Write([]byte(res))
				require.NoError(t, err)
			},
			expectedParams: map[string]interface{}{
				"page": 0,
			},
			expectedResult: []cdcexchange.Trade{
				{
					Side:           cdcexchange.OrderSideSell,
					InstrumentName: "ETH_CRO",
					Fee:            0.014,
					TradeID:        "367107655537806900",
					CreateTime:     cdctime.Time(now),
					TradedPrice:    7,
					TradedQuantity: 1,
					FeeCurrency:    "CRO",
					OrderID:        "367107623521528450",
				},
			},
		},
		{
			name: "successfully gets all trades between timestamps",
			args: args{
				req: cdcexchange.GetTradesRequest{
					Start:    now,
					End:      now.Add(time.Hour),
					PageSize: 1,
					Page:     2,
				},
			},
			handlerFunc: func(w http.ResponseWriter, r *http.Request) {
				assert.Contains(t, r.URL.Path, cdcexchange.MethodGetTrades)
				t.Cleanup(func() { require.NoError(t, r.Body.Close()) })

				var body api.Request
				require.NoError(t, json.NewDecoder(r.Body).Decode(&body))

				assert.Equal(t, cdcexchange.MethodGetTrades, body.Method)
				assert.Equal(t, id, body.ID)
				assert.Equal(t, apiKey, body.APIKey)
				assert.Equal(t, now.UnixMilli(), body.Nonce)
				assert.Equal(t, signature, body.Signature)
				assert.Equal(t, float64(2), body.Params["page"])
				assert.Equal(t, float64(1), body.Params["page_size"])
				assert.Equal(t, float64(now.UnixMilli()), body.Params["start_ts"])
				assert.Equal(t, float64(now.Add(time.Hour).UnixMilli()), body.Params["end_ts"])

				res := fmt.Sprintf(`{
							"id": 0,
							"method":"",
							"code":0,
							"result":{
								"trade_list":[
									{
										"side": "SELL",
										"instrument_name": "ETH_CRO",
										"fee": 0.014,
										"trade_id": "367107655537806900",
										"create_time": %d,
										"traded_price": 7,
										"traded_quantity": 1,
										"fee_currency": "CRO",
										"order_id": "367107623521528450"
								   }
								]
							}
						}`, now.UnixMilli())

				_, err := w.Write([]byte(res))
				require.NoError(t, err)
			},
			expectedParams: map[string]interface{}{
				"start_ts":  now.UnixMilli(),
				"end_ts":    now.Add(time.Hour).UnixMilli(),
				"page_size": 1,
				"page":      2,
			},
			expectedResult: []cdcexchange.Trade{
				{
					Side:           cdcexchange.OrderSideSell,
					InstrumentName: "ETH_CRO",
					Fee:            0.014,
					TradeID:        "367107655537806900",
					CreateTime:     cdctime.Time(now),
					TradedPrice:    7,
					TradedQuantity: 1,
					FeeCurrency:    "CRO",
					OrderID:        "367107623521528450",
				},
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
				Method:    cdcexchange.MethodGetTrades,
				Timestamp: now.UnixMilli(),
				Params:    tt.expectedParams,
			}).Return(signature, nil)

			res, err := client.GetTrades(ctx, tt.req)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedResult, res)
		})
	}
}
