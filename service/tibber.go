package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// TibberClient represents a client for the Tibber GraphQL API
type TibberClient struct {
	apiKey  string
	baseURL string
	httpClient *http.Client
}

// NewTibberClient creates a new Tibber client
func NewTibberClient(apiKey string) *TibberClient {
	return &TibberClient{
		apiKey:  apiKey,
		baseURL: "https://api.tibber.com/v1-beta/gql",
		// Alternative: baseURL: "https://api.tibber.com/graphql",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// PriceInfo represents price information for a specific time
type PriceInfo struct {
	Total     float64   `json:"total"`
	Energy    float64   `json:"energy"`
	Tax       float64   `json:"tax"`
	StartsAt  time.Time `json:"startsAt"`
	Level     string    `json:"level"`
}

// PriceResponse represents the response from Tibber price queries
type PriceResponse struct {
	Data   *PriceData   `json:"data"`
	Errors []GraphQLError `json:"errors"`
}

type PriceData struct {
	Home *HomeData `json:"home"`
}

type HomeData struct {
	CurrentSubscription *Subscription `json:"currentSubscription"`
	PriceInfo           *PriceInfoData `json:"priceInfo"`
}

type Subscription struct {
	PriceInfo []PriceInfo `json:"priceInfo"`
}

type PriceInfoData struct {
	Today    []PriceInfo `json:"today"`
	Tomorrow []PriceInfo `json:"tomorrow"`
}

type GraphQLError struct {
	Message string `json:"message"`
}

// GetCurrentPrice returns the current electricity price
func (t *TibberClient) GetCurrentPrice(ctx context.Context) (*PriceInfo, error) {
	query := `
		query {
			viewer {
				homes {
					currentSubscription {
						priceInfo {
							current {
								total
								energy
								tax
								startsAt
							}
						}
					}
				}
			}
		}
	`

	// Make request directly
	reqBody := map[string]interface{}{
		"query": query,
	}
	jsonBody, _ := json.Marshal(reqBody)

	req, _ := http.NewRequestWithContext(ctx, "POST", t.baseURL, bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+t.apiKey)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	slog.Info("GetCurrentPrice raw response", "body", string(body[:min(500, len(body))]))

	var result struct {
		Data struct {
			Viewer struct {
				Homes []struct {
					CurrentSubscription struct {
						PriceInfo struct {
							Current PriceInfo `json:"current"`
						} `json:"priceInfo"`
					} `json:"currentSubscription"`
				} `json:"homes"`
			} `json:"viewer"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		slog.Info("GetCurrentPrice unmarshal error", "error", err.Error())
		return nil, fmt.Errorf("unmarshal error: %w", err)
	}

	if len(result.Data.Viewer.Homes) == 0 {
		return nil, fmt.Errorf("no homes found")
	}

	home := result.Data.Viewer.Homes[0]
	return &home.CurrentSubscription.PriceInfo.Current, nil
}

// GetTodayPrices returns today's electricity prices
func (t *TibberClient) GetTodayPrices(ctx context.Context) ([]PriceInfo, error) {
	query := `
		query {
			viewer {
				homes {
					currentSubscription {
						priceInfo {
							today {
								total
								energy
								tax
								startsAt
							}
						}
					}
				}
			}
		}
	`

	reqBody := map[string]interface{}{"query": query}
	jsonBody, _ := json.Marshal(reqBody)
	req, _ := http.NewRequestWithContext(ctx, "POST", t.baseURL, bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+t.apiKey)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Data struct {
			Viewer struct {
				Homes []struct {
					CurrentSubscription struct {
						PriceInfo struct {
							Today []PriceInfo `json:"today"`
						} `json:"priceInfo"`
					} `json:"currentSubscription"`
				} `json:"homes"`
			} `json:"viewer"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if len(result.Data.Viewer.Homes) == 0 {
		return nil, fmt.Errorf("no homes found")
	}

	return result.Data.Viewer.Homes[0].CurrentSubscription.PriceInfo.Today, nil
}

// GetTomorrowPrices returns tomorrow's electricity prices (if available)
func (t *TibberClient) GetTomorrowPrices(ctx context.Context) ([]PriceInfo, error) {
	query := `
		query {
			viewer {
				homes {
					currentSubscription {
						priceInfo {
							tomorrow {
								total
								energy
								tax
								startsAt
							}
						}
					}
				}
			}
		}
	`

	reqBody := map[string]interface{}{"query": query}
	jsonBody, _ := json.Marshal(reqBody)
	req, _ := http.NewRequestWithContext(ctx, "POST", t.baseURL, bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+t.apiKey)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Data struct {
			Viewer struct {
				Homes []struct {
					CurrentSubscription struct {
						PriceInfo struct {
							Tomorrow []PriceInfo `json:"tomorrow"`
						} `json:"priceInfo"`
					} `json:"currentSubscription"`
				} `json:"homes"`
			} `json:"viewer"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if len(result.Data.Viewer.Homes) == 0 {
		return nil, fmt.Errorf("no homes found")
	}

	return result.Data.Viewer.Homes[0].CurrentSubscription.PriceInfo.Tomorrow, nil
}

// GetPrices returns both today's and tomorrow's prices
func (t *TibberClient) GetPrices(ctx context.Context) (*PriceInfoData, error) {
	query := `
		query {
			viewer {
				homes {
					currentSubscription {
						priceInfo {
							today {
								total
								energy
								tax
								startsAt
							}
							tomorrow {
								total
								energy
								tax
								startsAt
							}
						}
					}
				}
			}
		}
	`

	reqBody := map[string]interface{}{"query": query}
	jsonBody, _ := json.Marshal(reqBody)
	req, _ := http.NewRequestWithContext(ctx, "POST", t.baseURL, bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+t.apiKey)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var response struct {
		Data struct {
			Viewer struct {
				Homes []struct {
					CurrentSubscription struct {
						PriceInfo PriceInfoData `json:"priceInfo"`
					} `json:"currentSubscription"`
				} `json:"homes"`
			} `json:"viewer"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	if len(response.Data.Viewer.Homes) == 0 {
		return nil, fmt.Errorf("no homes found")
	}

	return &response.Data.Viewer.Homes[0].CurrentSubscription.PriceInfo, nil
}

// ConsumptionData represents consumption data
type ConsumptionData struct {
	Nodes []ConsumptionNode `json:"nodes"`
}

type ConsumptionNode struct {
	From           time.Time `json:"from"`
	To             time.Time `json:"to"`
	UnitPrice      float64   `json:"unitPrice"`
	UnitPriceVAT   float64   `json:"unitPriceVAT"`
	Consumption    float64   `json:"consumption"`
	ConsumptionUnit string   `json:"consumptionUnit"`
	Cost           float64   `json:"cost"`
}

// GetConsumption returns consumption data for a specific time range
func (t *TibberClient) GetConsumption(ctx context.Context, from, to time.Time) ([]ConsumptionNode, error) {
	return t.GetConsumptionByTimeRange(ctx, from.Format(time.RFC3339), to.Format(time.RFC3339))
}

// GetConsumptionByTimeRange returns consumption data for a specific time range using string format
func (t *TibberClient) GetConsumptionByTimeRange(ctx context.Context, fromStr, toStr string) ([]ConsumptionNode, error) {
	query := `
		query GetConsumption($from: ISO8601DateTime!, $to: ISO8601DateTime!) {
			viewer {
				homes {
					consumption(resolution: HOURLY, from: $from, to: $to) {
						nodes {
							from
							to
							unitPrice
							unitPriceVAT
							consumption
							consumptionUnit
							cost
						}
					}
				}
			}
		}
	`

	reqBody := map[string]interface{}{
		"query": query,
		"variables": map[string]interface{}{
			"from": fromStr,
			"to":   toStr,
		},
	}
	jsonBody, _ := json.Marshal(reqBody)
	req, _ := http.NewRequestWithContext(ctx, "POST", t.baseURL, bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+t.apiKey)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var response struct {
		Data struct {
			Viewer struct {
				Homes []struct {
					Consumption ConsumptionData `json:"consumption"`
				} `json:"homes"`
			} `json:"viewer"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	if len(response.Data.Viewer.Homes) == 0 {
		return nil, fmt.Errorf("no homes found")
	}

	return response.Data.Viewer.Homes[0].Consumption.Nodes, nil
}

// Home represents home information
type Home struct {
	ID                string `json:"id"`
	TimeZone          string `json:"timeZone"`
	Address           Address `json:"address"`
	Features          Features `json:"features"`
}

type Address struct {
	Address1 string `json:"address1"`
	City     string `json:"city"`
	PostalCode string `json:"postalCode"`
	Country  string `json:"country"`
}

type Features struct {
	RealTimeConsumptionEnabled bool `json:"realTimeConsumptionEnabled"`
}

// GetHome returns information about the user's home
func (t *TibberClient) GetHome(ctx context.Context) (*Home, error) {
	query := `
		query {
			viewer {
				homes {
					id
					timeZone
					address {
						address1
						city
						postalCode
						country
					}
					features {
						realTimeConsumptionEnabled
					}
				}
			}
		}
	`

	reqBody := map[string]interface{}{"query": query}
	jsonBody, _ := json.Marshal(reqBody)
	req, _ := http.NewRequestWithContext(ctx, "POST", t.baseURL, bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+t.apiKey)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var response struct {
		Data struct {
			Viewer struct {
				Homes []Home `json:"homes"`
			} `json:"viewer"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	if len(response.Data.Viewer.Homes) == 0 {
		return nil, fmt.Errorf("no homes found")
	}

	home := response.Data.Viewer.Homes[0]
	return &home, nil
}

// executeQuery executes a GraphQL query against the Tibber API
func (t *TibberClient) executeQuery(ctx context.Context, query string, variables map[string]interface{}, result interface{}) error {
	reqBody := map[string]interface{}{
		"query": query,
	}
	if variables != nil {
		reqBody["variables"] = variables
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", t.baseURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+t.apiKey)
	req.Header.Set("User-Agent", "EcoFlow-SmartCharging/1.0")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body for debugging
	bodyBytes, _ := io.ReadAll(resp.Body)
	resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	slog.Info("Tibber raw response", "status", resp.StatusCode, "bodyLen", len(bodyBytes))

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d, response: %s", resp.StatusCode, string(bodyBytes))
	}

	var response struct {
		Data   json.RawMessage `json:"data"`
		Errors []GraphQLError  `json:"errors"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if len(response.Errors) > 0 {
		return fmt.Errorf("GraphQL error: %s", response.Errors[0].Message)
	}

	maxLen := 500
	if len(response.Data) < maxLen {
		maxLen = len(response.Data)
	}
	slog.Info("executeQuery: response data length", "len", len(response.Data), "dataString", string(response.Data[:maxLen]))

	if response.Data != nil {
		slog.Info("executeQuery: about to unmarshal into result", "resultType", fmt.Sprintf("%T", result))
		if err := json.Unmarshal(response.Data, result); err != nil {
			slog.Info("executeQuery: unmarshal error", "error", err.Error())
			return fmt.Errorf("failed to unmarshal data: %w", err)
		}
		slog.Info("executeQuery: unmarshal succeeded")
		// After unmarshal, check what's in result
		if resultMap, ok := result.(*struct { Data struct { Viewer json.RawMessage } }); ok {
			slog.Info("executeQuery: after unmarshal", "viewerLen", len(resultMap.Data.Viewer))
		}
	}

	// Debug: print the result
	slog.Info("executeQuery: result type", "type", fmt.Sprintf("%T", result))

	return nil
}

// DebugViewer returns raw viewer data for debugging
func (t *TibberClient) DebugViewer(ctx context.Context) (map[string]interface{}, error) {
	query := `
		query {
			viewer {
				homes {
					address {
						address1
						address2
						address3
						postalCode
						city
						country
						latitude
						longitude
					}
				}
			}
		}
	`

	reqBody := map[string]interface{}{"query": query}
	jsonBody, _ := json.Marshal(reqBody)
	req, _ := http.NewRequestWithContext(ctx, "POST", t.baseURL, bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+t.apiKey)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// Use a simple map to capture the result
	var result map[string]interface{}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	// Check if there's data
	if result == nil {
		return map[string]interface{}{
			"message": "No data returned",
		}, nil
	}

	return result, nil
}

// LiveMeasurement represents live consumption data from Tibber Pulse
type LiveMeasurement struct {
	Timestamp              time.Time `json:"timestamp"`
	Power                  float64   `json:"power"`
	AccumulatedConsumption float64   `json:"accumulatedConsumption"`
	AccumulatedCost        float64   `json:"accumulatedCost"`
	Currency               string    `json:"currency"`
	MinPower               float64   `json:"minPower"`
	AveragePower           float64   `json:"averagePower"`
	MaxPower               float64   `json:"maxPower"`
}

// GetLiveMeasurement returns live consumption data for the user's home
func (t *TibberClient) GetLiveMeasurement(ctx context.Context) (*LiveMeasurement, error) {
	// First get the home ID
	home, err := t.GetHome(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get home: %w", err)
	}

	query := `
		query GetLiveMeasurement($homeId: ID!) {
			liveMeasurement(homeId: $homeId) {
				timestamp
				power
				accumulatedConsumption
				accumulatedCost
				currency
				minPower
				averagePower
				maxPower
			}
		}
	`

	reqBody := map[string]interface{}{
		"query": query,
		"variables": map[string]interface{}{
			"homeId": home.ID,
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", t.baseURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+t.apiKey)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var response struct {
		Data struct {
			LiveMeasurement LiveMeasurement `json:"liveMeasurement"`
		} `json:"data"`
		Errors []GraphQLError `json:"errors"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(response.Errors) > 0 {
		return nil, fmt.Errorf("GraphQL error: %s", response.Errors[0].Message)
	}

	return &response.Data.LiveMeasurement, nil
}
