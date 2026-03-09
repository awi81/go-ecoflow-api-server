package service

import (
	"context"
	"fmt"
	"math"
	"time"
)

// ChargingConfig holds the configuration for smart charging
type ChargingConfig struct {
	MinSOC           int     // Minimum SOC to discharge to (0-100)
	MaxSOC           int     // Maximum SOC to charge to (0-100)
	MaxPriceToCharge float64 // Maximum price per kWh to allow charging
	MinPriceToDischarge float64 // Minimum price per kWh to allow discharging
	ChargingPowerWatts int     // Charging power in watts
	DischargingPowerWatts int  // Discharging power in watts
	PVSurplusWatts    int     // Current PV surplus in watts
}

// ChargingAction represents the recommended charging action
type ChargingAction struct {
	Action       string    // "charge", "discharge", "idle", "wait"
	Reason       string    // Explanation for the action
	PowerWatts   int       // Power level to use
	SOCLimit     int       // Target SOC limit
	EstimatedCost float64  // Estimated cost/earnings
	Until        *time.Time // When to re-evaluate
}

// ChargingStrategy determines the optimal charging action based on prices and config
func (t *TibberClient) ChargingStrategy(ctx context.Context, config ChargingConfig) (*ChargingAction, error) {
	// Get current and upcoming prices
	prices, err := t.GetPrices(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get prices: %w", err)
	}

	// Get current price
	currentPrice, err := t.GetCurrentPrice(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get current price: %w", err)
	}

	// Determine action based on price and PV surplus
	action := calculateAction(currentPrice, prices, config)
	return action, nil
}

// calculateAction determines the optimal charging action
func calculateAction(currentPrice *PriceInfo, prices *PriceInfoData, config ChargingConfig) *ChargingAction {
	now := time.Now()

	// Find cheapest hours in the next 24 hours
	var upcomingPrices []PriceInfo
	if prices != nil {
		upcomingPrices = append(upcomingPrices, prices.Today...)
		upcomingPrices = append(upcomingPrices, prices.Tomorrow...)
	}

	// Check if we have PV surplus
	hasPVSurplus := config.PVSurplusWatts > 100 // At least 100W surplus

	// Decision logic
	switch {
	// Case 1: PV surplus available - always charge if not at max
	case hasPVSurplus:
		return &ChargingAction{
			Action:     "charge",
			Reason:     "PV-Überschuss verfügbar - günstig laden",
			PowerWatts: config.PVSurplusWatts,
			SOCLimit:   config.MaxSOC,
			EstimatedCost: -0.01, // Negative = earning
		}

	// Case 2: Very cheap price - charge if not at max
	case currentPrice.Total <= config.MaxPriceToCharge*0.5:
		return &ChargingAction{
			Action:     "charge",
			Reason:     "Sehr günstiger Strompreis",
			PowerWatts: config.ChargingPowerWatts,
			SOCLimit:   config.MaxSOC,
			EstimatedCost: currentPrice.Total * float64(config.ChargingPowerWatts) / 1000,
			Until:      calculateNextActionTime(upcomingPrices, now, config.MaxPriceToCharge),
		}

	// Case 3: Below max price threshold - consider charging
	case currentPrice.Total <= config.MaxPriceToCharge:
		// Check if there's a cheaper period coming
		nextCheaperHour := findNextCheaperHour(upcomingPrices, currentPrice.Total, now)
		if nextCheaperHour != nil && nextCheaperHour.Sub(now) > 2*time.Hour {
			return &ChargingAction{
				Action:     "wait",
				Reason:     "Günstigerer Preis in Kürze verfügbar",
				PowerWatts: 0,
				SOCLimit:   config.MinSOC,
				EstimatedCost: 0,
				Until:      nextCheaperHour,
			}
		}
		return &ChargingAction{
			Action:     "charge",
			Reason:     "Preis unter Schwellenwert",
			PowerWatts: config.ChargingPowerWatts,
			SOCLimit:   config.MaxSOC,
			EstimatedCost: currentPrice.Total * float64(config.ChargingPowerWatts) / 1000,
		}

	// Case 4: Above discharge threshold and price is high - discharge
	case currentPrice.Total >= config.MinPriceToDischarge:
		return &ChargingAction{
			Action:     "discharge",
			Reason:     "Hoher Strompreis - Akku verkaufen",
			PowerWatts: config.DischargingPowerWatts,
			SOCLimit:   config.MinSOC,
			EstimatedCost: -currentPrice.Total * float64(config.DischargingPowerWatts) / 1000,
		}

	// Default: Idle
	default:
		return &ChargingAction{
			Action:     "idle",
			Reason:     "Preis im normalen Bereich",
			PowerWatts: 0,
			SOCLimit:   config.MinSOC,
			EstimatedCost: 0,
		}
	}
}

// findNextCheaperHour finds the next hour with cheaper prices
func findNextCheaperHour(prices []PriceInfo, currentPrice float64, now time.Time) *time.Time {
	for _, p := range prices {
		pTime, err := time.Parse(time.RFC3339, p.StartsAt.Format(time.RFC3339))
		if err != nil {
			continue
		}
		if pTime.After(now) && p.Total < currentPrice*0.8 {
			return &pTime
		}
	}
	return nil
}

// calculateNextActionTime calculates when to re-evaluate
func calculateNextActionTime(prices []PriceInfo, now time.Time, maxPrice float64) *time.Time {
	for _, p := range prices {
		pTime, err := time.Parse(time.RFC3339, p.StartsAt.Format(time.RFC3339))
		if err != nil {
			continue
		}
		if pTime.After(now) && p.Total > maxPrice {
			return &pTime
		}
	}
	// Default: re-evaluate in 1 hour
	nextHour := now.Add(1 * time.Hour)
	return &nextHour
}

// CalculateOptimalChargingTime calculates the best time windows for charging
func (t *TibberClient) CalculateOptimalChargingTime(ctx context.Context, hoursNeeded int) ([]time.Time, error) {
	prices, err := t.GetPrices(ctx)
	if err != nil {
		return nil, err
	}

	var allPrices []PriceInfo
	allPrices = append(allPrices, prices.Today...)
	allPrices = append(allPrices, prices.Tomorrow...)

	if len(allPrices) == 0 {
		return nil, fmt.Errorf("no price data available")
	}

	// Sort by price (cheapest first)
	for i := 0; i < len(allPrices)-1; i++ {
		for j := i + 1; j < len(allPrices); j++ {
			if allPrices[j].Total < allPrices[i].Total {
				allPrices[i], allPrices[j] = allPrices[j], allPrices[i]
			}
		}
	}

	// Take the cheapest hours
	var optimalTimes []time.Time
	count := 0
	for _, p := range allPrices {
		if count >= hoursNeeded {
			break
		}
		optimalTimes = append(optimalTimes, p.StartsAt)
		count++
	}

	return optimalTimes, nil
}

// CalculateSavings calculates potential savings from smart charging
func (t *TibberClient) CalculateSavings(ctx context.Context, config ChargingConfig) (map[string]float64, error) {
	prices, err := t.GetPrices(ctx)
	if err != nil {
		return nil, err
	}

	var allPrices []PriceInfo
	allPrices = append(allPrices, prices.Today...)
	allPrices = append(allPrices, prices.Tomorrow...)

	if len(allPrices) == 0 {
		return nil, fmt.Errorf("no price data available")
	}

	// Calculate average price
	var totalPrice float64
	for _, p := range allPrices {
		totalPrice += p.Total
	}
	averagePrice := totalPrice / float64(len(allPrices))

	// Calculate potential savings
	// Assuming 10kWh daily charging
	dailyCapacity := 10.0 // kWh

	// Smart charging: only charge when price is below average
	var smartCost float64
	var dumbCost float64
	for _, p := range allPrices[:24] { // Today only
		if p.Total <= averagePrice {
			smartCost += p.Total * dailyCapacity / float64(len(allPrices[:24]))
		} else {
			smartCost += p.Total * dailyCapacity / float64(len(allPrices[:24]))
		}
		dumbCost += p.Total * dailyCapacity / float64(len(allPrices[:24]))
	}

	savingsPerDay := dumbCost - smartCost

	return map[string]float64{
		"averagePrice":    averagePrice,
		"smartCost":       smartCost,
		"dumbCost":        dumbCost,
		"savingsPerDay":   savingsPerDay,
		"savingsPerMonth": savingsPerDay * 30,
		"savingsPerYear":   savingsPerDay * 365,
	}, nil
}

// round rounds a float to the specified precision
func round(val float64, precision int) float64 {
	ratio := math.Pow(10, float64(precision))
	return math.Round(val*ratio) / ratio
}
