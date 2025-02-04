package cdcexchange_test

import (
	"context"
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
)

func TestClient_GetBook_Error(t *testing.T) {
	const (
		apiKey    = "some api key"
		secretKey = "some secret key"
	)
	testErr := errors.New("some error")

	tests := []struct {
		name        string
		client      http.Client
		responseErr error
		expectedErr error
	}{
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
				now   = time.Now()
				clock = clockwork.NewFakeClockAt(now)
			)

			client, err := cdcexchange.New(apiKey, secretKey,
				cdcexchange.WithClock(clock),
				cdcexchange.WithHTTPClient(&tt.client),
			)
			require.NoError(t, err)

			books, err := client.GetBook(ctx, "some instrument", 1)
			require.Error(t, err)

			assert.Empty(t, books)

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

func TestClient_GetBook_Success(t *testing.T) {
	const (
		apiKey     = "some api key"
		secretKey  = "some secret key"
		instrument = "some instrument"
		depth      = 100
	)
	now := time.Now().Round(time.Second)

	type args struct {
		instrument string
		depth      int
	}
	tests := []struct {
		name        string
		handlerFunc func(w http.ResponseWriter, r *http.Request)
		args
		expectedResult cdcexchange.BookResult
	}{
		{
			name: "returns books for specific instrument",
			args: args{
				instrument: instrument,
				depth:      depth,
			},
			handlerFunc: func(w http.ResponseWriter, r *http.Request) {
				assert.Contains(t, r.URL.Path, cdcexchange.MethodGetBook)
				assert.Equal(t, http.MethodGet, r.Method)
				t.Cleanup(func() { require.NoError(t, r.Body.Close()) })

				require.Empty(t, r.Body)

				instrumentName := r.URL.Query().Get("instrument_name")
				assert.Equal(t, instrument, instrumentName)

				depthParam := r.URL.Query().Get("depth")
				assert.Equal(t, fmt.Sprintf("%d", depth), depthParam)

				res := fmt.Sprintf(`{
							"id": 0,
							"method":"",
							"code":0,
							"result":{
								"bids":[[9668.44,0.006325,1.0]],
								"asks":[[9697.0,0.68251,1.0]],
								"t": %d
							}
						}`, now.UnixMilli())

				_, err := w.Write([]byte(res))
				require.NoError(t, err)
			},
			expectedResult: cdcexchange.BookResult{
				//Bids:      [][]float64{{9668.44, 0.006325, 1.0}},
				//Asks:      [][]float64{{9697.0, 0.68251, 1.0}},
				//Timestamp: cdctime.Time(now),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl, ctx := gomock.WithContext(context.Background(), t)
			t.Cleanup(ctrl.Finish)

			var (
				clock = clockwork.NewFakeClockAt(now)
			)

			s := httptest.NewServer(http.HandlerFunc(tt.handlerFunc))
			t.Cleanup(s.Close)

			client, err := cdcexchange.New(apiKey, secretKey,
				cdcexchange.WithClock(clock),
				cdcexchange.WithHTTPClient(s.Client()),
				cdcexchange.WithBaseURL(fmt.Sprintf("%s/", s.URL)),
			)
			require.NoError(t, err)

			res, err := client.GetBook(ctx, tt.instrument, tt.depth)
			require.NoError(t, err)

			assert.Equal(t, &tt.expectedResult, res)
		})
	}
}
