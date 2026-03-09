package handlers

import (
	"context"
	"encoding/json"

	"github.com/go-chi/chi/v5"
	"go-ecoflow-api-server/constants"
	"net/http"
)

type DeviceHandler struct {
	*BaseHandler
}

func NewDeviceHandler(baseHandler *BaseHandler) *DeviceHandler {
	return &DeviceHandler{baseHandler}
}

func (h *DeviceHandler) RegisterRoutes(router chi.Router) {
	validator := DefaultSerialNumberValidator()

	router.Get("/api/devices", h.GetDevicesList())

	// Apply serial number validation to routes with {serial_number} parameter
	router.With(SerialNumberValidationMiddleware(validator, "serial_number")).Route("/api/devices/{serial_number}", func(r chi.Router) {
		r.Get("/parameters", h.GetDeviceParametersAll())
		r.Post("/parameters/query", h.GetDeviceParametersQuery())
		r.Get("/soc", h.GetDeviceSOC())
	})
}

// GetDevicesList handles retrieving a list of devices
// @Summary Get a list of devices
// @Description Returns a list of all devices associated with the user
// @Tags Devices
// @Produce json
// @Success 200 {object} SuccessResponse "List of devices retrieved successfully"
// @Failure 500 {object} ErrorResponse "Error retrieving device list"
// @Router /api/devices [get]
func (h *DeviceHandler) GetDevicesList() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		client, ok := h.GetEcoflowClientOrRespondWithError(r, w)
		if !ok {
			return
		}

		ecoflowResponse, err := client.GetDeviceList(context.Background())
		if err != nil {
			h.RespondWithError(w, http.StatusInternalServerError, constants.ErrGetDevicesList, err.Error(), nil)
			return
		}

		// Convert to JSON and back to extract nested data
		jsonBytes, _ := json.Marshal(ecoflowResponse)
		var ecoflowData map[string]interface{}
		json.Unmarshal(jsonBytes, &ecoflowData)

		// Extract the actual device list from the nested response
		// Ecoflow returns: {code: "0", message: "Success", data: [...]}
		// We need to return just the data array
		if dataList, exists := ecoflowData["data"]; exists {
			h.RespondWithSuccess(w, dataList)
			return
		}

		h.RespondWithSuccess(w, ecoflowResponse)
	}
}

// GetDeviceParametersAll handles retrieving all parameters for a specific device
// @Summary Get all parameters for a device
// @Description Retrieves all available parameters for a device using its serial number
// @Tags Devices
// @Produce json
// @Param serial_number path string true "Device Serial Number"
// @Success 200 {object} SuccessResponse "Parameters retrieved successfully"
// @Failure 500 {object} ErrorResponse "Error retrieving device parameters"
// @Router /api/devices/{serial_number}/parameters [get]
func (h *DeviceHandler) GetDeviceParametersAll() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		client, ok := h.GetEcoflowClientOrRespondWithError(r, w)
		if !ok {
			return
		}

		sn := r.PathValue("serial_number")
		ecoflowResponse, err := client.GetDeviceAllParameters(context.Background(), sn)
		if err != nil {
			h.RespondWithError(w, http.StatusInternalServerError, constants.ErrGetAllDeviceParameters, err.Error(), map[string]string{
				"serial_number": sn,
			})
			return
		}
		h.RespondWithSuccess(w, ecoflowResponse)
	}
}

// QueryParametersRequest represents the request body for querying specific parameters of a device
type QueryParametersRequest struct {
	Parameters []string `json:"parameters"`
}

// GetDeviceParametersQuery handles querying specific parameters for a device
// @Summary Query specific parameters for a device
// @Description Queries specific parameters for a device using its serial number
// @Tags Devices
// @Accept json
// @Produce json
// @Param serial_number path string true "Device Serial Number"
// @Param parameters body QueryParametersRequest true "List of parameters to query"
// @Success 200 {object} SuccessResponse "Requested parameters retrieved successfully"
// @Failure 400 {object} ErrorResponse "Error Invalid JSON Body"
// @Failure 500 {object} ErrorResponse "Error retrieving device parameters"
// @Router /api/devices/{serial_number}/parameters/query [post]
func (h *DeviceHandler) GetDeviceParametersQuery() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		sn := r.PathValue("serial_number")

		var requestBody struct {
			Parameters []string `json:"parameters"`
		}

		err := json.NewDecoder(r.Body).Decode(&requestBody)
		if err != nil {
			h.RespondWithError(w, http.StatusBadRequest, constants.ErrInvalidJsonBody, "Invalid JSON Body", map[string]string{
				"serial_number": sn,
				"error":         err.Error(),
			})
			return
		}

		if len(requestBody.Parameters) == 0 {
			h.RespondWithError(w, http.StatusBadRequest, constants.ErrInvalidJsonBody, "No parameters provided", map[string]string{
				"serial_number": sn,
			})
			return
		}

		client, ok := h.GetEcoflowClientOrRespondWithError(r, w)
		if !ok {
			return
		}

		ecoflowResponse, err := client.GetDeviceParameters(context.Background(), sn, requestBody.Parameters)
		if err != nil {
			h.RespondWithError(w, http.StatusInternalServerError, constants.ErrGetDeviceParameters, err.Error(), map[string]string{
				"serial_number": sn,
			})
			return
		}
		h.RespondWithSuccess(w, ecoflowResponse)
	}
}

// GetDeviceSOC handles retrieving the battery state of charge for a specific device
// @Summary Get battery SOC for a device
// @Description Returns the current battery state of charge (SOC) for a device
// @Tags Devices
// @Produce json
// @Param serial_number path string true "Device Serial Number"
// @Success 200 {object} SuccessResponse "SOC retrieved successfully"
// @Failure 500 {object} ErrorResponse "Error retrieving SOC"
// @Router /api/devices/{serial_number}/soc [get]
func (h *DeviceHandler) GetDeviceSOC() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		client, ok := h.GetEcoflowClientOrRespondWithError(r, w)
		if !ok {
			return
		}

		sn := r.PathValue("serial_number")

		// Try to get specific parameters that include SOC
		// Common parameter names for battery state: emsParams, sysInfo, pd, etc.
		paramsToTry := [][]string{
			{"emsParams", "soc"},
			{"sysInfo", "soc"},
			{"pd", "soc"},
			{"bms", "soc"},
		}

		for _, params := range paramsToTry {
			ecoflowResponse, err := client.GetDeviceParameters(context.Background(), sn, params)
			if err != nil {
				continue
			}

			// Parse response to find SOC value
			jsonBytes, _ := json.Marshal(ecoflowResponse)
			var data map[string]interface{}
			json.Unmarshal(jsonBytes, &data)

			// Try to extract SOC from response
			if soc := extractSOC(data); soc != nil {
				h.RespondWithSuccess(w, map[string]interface{}{
					"serialNumber": sn,
					"soc":          soc,
				})
				return
			}
		}

		// If specific params failed, try all parameters
		ecoflowResponse, err := client.GetDeviceAllParameters(context.Background(), sn)
		if err != nil {
			h.RespondWithError(w, http.StatusInternalServerError, constants.ErrGetAllDeviceParameters, err.Error(), map[string]string{
				"serial_number": sn,
			})
			return
		}

		jsonBytes, _ := json.Marshal(ecoflowResponse)
		var data map[string]interface{}
		json.Unmarshal(jsonBytes, &data)

		if soc := extractSOC(data); soc != nil {
			h.RespondWithSuccess(w, map[string]interface{}{
				"serialNumber": sn,
				"soc":          soc,
			})
			return
		}

		h.RespondWithError(w, http.StatusNotFound, constants.ErrGetAllDeviceParameters, "SOC not found in device parameters", map[string]string{
			"serial_number": sn,
		})
	}
}

// extractSOC tries to extract SOC value from various response structures
func extractSOC(data map[string]interface{}) interface{} {
	// Try direct soc
	if v, ok := data["soc"].(float64); ok {
		return v
	}

	// Try nested data
	if dataVal, ok := data["data"].(map[string]interface{}); ok {
		if v, ok := dataVal["soc"].(float64); ok {
			return v
		}
		// Try emsParams
		if ems, ok := dataVal["emsParams"].(map[string]interface{}); ok {
			if v, ok := ems["soc"].(float64); ok {
				return v
			}
		}
		// Try sysInfo
		if sys, ok := dataVal["sysInfo"].(map[string]interface{}); ok {
			if v, ok := sys["soc"].(float64); ok {
				return v
			}
		}
		// Try pd
		if pd, ok := dataVal["pd"].(map[string]interface{}); ok {
			if v, ok := pd["soc"].(float64); ok {
				return v
			}
		}
	}

	// Try root level data array
	if dataList, ok := data["data"].([]interface{}); ok && len(dataList) > 0 {
		if item, ok := dataList[0].(map[string]interface{}); ok {
			if v, ok := item["soc"].(float64); ok {
				return v
			}
		}
	}

	return nil
}
