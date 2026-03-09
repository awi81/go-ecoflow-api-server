package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"go-ecoflow-api-server/constants"
	"go-ecoflow-api-server/service"
)

// TibberHandler handles Tibber API requests
type TibberHandler struct {
	*BaseHandler
	getTibberClient func(r *http.Request) (*service.TibberClient, error)
}

// NewTibberHandler creates a new Tibber handler
func NewTibberHandler(baseHandler *BaseHandler, getTibberClient func(r *http.Request) (*service.TibberClient, error)) *TibberHandler {
	return &TibberHandler{
		BaseHandler:     baseHandler,
		getTibberClient: getTibberClient,
	}
}

// GetTibberClientOrRespondWithError tries to get the Tibber client from the request
// and responds with an error if it fails
func (h *TibberHandler) GetTibberClientOrRespondWithError(w http.ResponseWriter, r *http.Request) (*service.TibberClient, bool) {
	client, err := h.getTibberClient(r)
	if err != nil {
		h.RespondWithError(w, http.StatusBadRequest, constants.ErrUnauthorized, err.Error(), nil)
		return nil, false
	}
	return client, true
}

// RegisterRoutes registers the Tibber routes
func (h *TibberHandler) RegisterRoutes(router chi.Router) {
	router.Route("/api/tibber", func(r chi.Router) {
		r.Get("/prices/current", h.GetCurrentPrice())
		r.Get("/prices/today", h.GetTodayPrices())
		r.Get("/prices/tomorrow", h.GetTomorrowPrices())
		r.Get("/prices", h.GetAllPrices())
		r.Get("/home", h.GetHomeInfo())
		r.Get("/consumption", h.GetConsumption())
		r.Get("/live", h.GetLiveMeasurement())
		r.Get("/debug", h.DebugViewer())
	})
}

// GetCurrentPrice handles retrieving the current electricity price
// @Summary Get current electricity price
// @Description Returns the current electricity price from Tibber
// @Tags Tibber
// @Produce json
// @Success 200 {object} SuccessResponse "Current price retrieved successfully"
// @Failure 500 {object} ErrorResponse "Error retrieving current price"
// @Router /api/tibber/prices/current [get]
func (h *TibberHandler) GetCurrentPrice() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		client, ok := h.GetTibberClientOrRespondWithError(w, r)
		if !ok {
			return
		}

		price, err := client.GetCurrentPrice(r.Context())
		if err != nil {
			h.RespondWithError(w, http.StatusInternalServerError, constants.ErrInternalServer, err.Error(), nil)
			return
		}
		h.RespondWithSuccess(w, price)
	}
}

// GetTodayPrices handles retrieving today's electricity prices
// @Summary Get today's electricity prices
// @Description Returns today's electricity prices from Tibber
// @Tags Tibber
// @Produce json
// @Success 200 {object} SuccessResponse "Today's prices retrieved successfully"
// @Failure 500 {object} ErrorResponse "Error retrieving today's prices"
// @Router /api/tibber/prices/today [get]
func (h *TibberHandler) GetTodayPrices() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		client, ok := h.GetTibberClientOrRespondWithError(w, r)
		if !ok {
			return
		}

		prices, err := client.GetTodayPrices(r.Context())
		if err != nil {
			h.RespondWithError(w, http.StatusInternalServerError, constants.ErrInternalServer, err.Error(), nil)
			return
		}
		h.RespondWithSuccess(w, prices)
	}
}

// GetTomorrowPrices handles retrieving tomorrow's electricity prices
// @Summary Get tomorrow's electricity prices
// @Description Returns tomorrow's electricity prices from Tibber (if available)
// @Tags Tibber
// @Produce json
// @Success 200 {object} SuccessResponse "Tomorrow's prices retrieved successfully"
// @Failure 500 {object} ErrorResponse "Error retrieving tomorrow's prices"
// @Router /api/tibber/prices/tomorrow [get]
func (h *TibberHandler) GetTomorrowPrices() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		client, ok := h.GetTibberClientOrRespondWithError(w, r)
		if !ok {
			return
		}

		prices, err := client.GetTomorrowPrices(r.Context())
		if err != nil {
			h.RespondWithError(w, http.StatusInternalServerError, constants.ErrInternalServer, err.Error(), nil)
			return
		}
		h.RespondWithSuccess(w, prices)
	}
}

// GetAllPrices handles retrieving both today's and tomorrow's prices
// @Summary Get all electricity prices
// @Description Returns both today's and tomorrow's electricity prices from Tibber
// @Tags Tibber
// @Produce json
// @Success 200 {object} SuccessResponse "All prices retrieved successfully"
// @Failure 500 {object} ErrorResponse "Error retrieving prices"
// @Router /api/tibber/prices [get]
func (h *TibberHandler) GetAllPrices() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		client, ok := h.GetTibberClientOrRespondWithError(w, r)
		if !ok {
			return
		}

		prices, err := client.GetPrices(r.Context())
		if err != nil {
			h.RespondWithError(w, http.StatusInternalServerError, constants.ErrInternalServer, err.Error(), nil)
			return
		}
		h.RespondWithSuccess(w, prices)
	}
}

// GetHomeInfo handles retrieving home information
// @Summary Get Tibber home info
// @Description Returns information about the user's Tibber home
// @Tags Tibber
// @Produce json
// @Success 200 {object} SuccessResponse "Home info retrieved successfully"
// @Failure 500 {object} ErrorResponse "Error retrieving home info"
// @Router /api/tibber/home [get]
func (h *TibberHandler) GetHomeInfo() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		client, ok := h.GetTibberClientOrRespondWithError(w, r)
		if !ok {
			return
		}

		home, err := client.GetHome(r.Context())
		if err != nil {
			h.RespondWithError(w, http.StatusInternalServerError, constants.ErrInternalServer, err.Error(), nil)
			return
		}
		h.RespondWithSuccess(w, home)
	}
}

// GetConsumption handles retrieving consumption data
// @Summary Get consumption data
// @Description Returns consumption data for a specified time range
// @Tags Tibber
// @Produce json
// @Param from query string false "Start time (RFC3339 format)"
// @Param to query string false "End time (RFC3339 format)"
// @Success 200 {object} SuccessResponse "Consumption data retrieved successfully"
// @Failure 500 {object} ErrorResponse "Error retrieving consumption data"
// @Router /api/tibber/consumption [get]
func (h *TibberHandler) GetConsumption() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		client, ok := h.GetTibberClientOrRespondWithError(w, r)
		if !ok {
			return
		}

		fromStr := r.URL.Query().Get("from")
		toStr := r.URL.Query().Get("to")

		consumption, err := client.GetConsumptionByTimeRange(r.Context(), fromStr, toStr)
		if err != nil {
			h.RespondWithError(w, http.StatusInternalServerError, constants.ErrInternalServer, err.Error(), nil)
			return
		}
		h.RespondWithSuccess(w, consumption)
	}
}

// GetLiveMeasurement handles retrieving live consumption data
// @Summary Get live consumption data
// @Description Returns live consumption data from Tibber Pulse
// @Tags Tibber
// @Produce json
// @Success 200 {object} SuccessResponse "Live measurement retrieved successfully"
// @Failure 500 {object} ErrorResponse "Error retrieving live measurement"
// @Router /api/tibber/live [get]
func (h *TibberHandler) GetLiveMeasurement() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		client, ok := h.GetTibberClientOrRespondWithError(w, r)
		if !ok {
			return
		}

		measurement, err := client.GetLiveMeasurement(r.Context())
		if err != nil {
			h.RespondWithError(w, http.StatusInternalServerError, constants.ErrInternalServer, err.Error(), nil)
			return
		}
		h.RespondWithSuccess(w, measurement)
	}
}

// DebugViewer handles debugging the Tibber API response
// @Summary Debug Tibber API
// @Description Returns raw viewer data for debugging
// @Tags Tibber
// @Produce json
// @Success 200 {object} SuccessResponse "Debug data retrieved"
// @Failure 500 {object} ErrorResponse "Error"
// @Router /api/tibber/debug [get]
func (h *TibberHandler) DebugViewer() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		client, ok := h.GetTibberClientOrRespondWithError(w, r)
		if !ok {
			return
		}

		data, err := client.DebugViewer(r.Context())
		if err != nil {
			h.RespondWithError(w, http.StatusInternalServerError, constants.ErrInternalServer, err.Error(), nil)
			return
		}
		h.RespondWithSuccess(w, data)
	}
}
