# Changelog

## [Unreleased]

### Added
- HTTPS/TLS support with configurable certificate and key paths
- CORS middleware with configurable allowed origins
- Serial number validation for all device endpoints
- Input validation for out_voltage parameter (100-240V range)
- Secure error handling - internal errors are logged but not exposed to clients
- Command line flags for TLS, CORS, and port configuration

### Security
- Added CORS protection to prevent unauthorized cross-origin requests
- Added TLS support for encrypted communication
- Improved error messages to prevent information leakage

## [0.0.3] - 2025-01-25

### Added
- Initial release
- Device listing endpoint
- Device parameters retrieval
- Power station AC/DC control
- Car charging control
- Charging speed configuration
- Standby parameter configuration
- Swagger documentation
- Docker support

### Security
- Bearer token authentication
- Secret token authentication
- Rate limiting (60 requests/minute)
- Sensitive header filtering in logs
