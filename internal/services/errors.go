package services

import (
	"errors"

	"github.com/sjperalta/fintera-api/pkg/logger"
)

// Common service errors
var (
	ErrNotFound            = errors.New("registro no encontrado")
	ErrInvalidPassword     = errors.New("contraseña inválida")
	ErrUnauthorized        = errors.New("no autorizado")
	ErrInvalidState        = errors.New("transición de estado inválida")
	ErrContractNotApproved = errors.New("no se puede aprobar el pago de reserva sin antes aprobar el contrato")
	ErrDuplicate           = errors.New("registro duplicado")
	ErrInvalidRecoveryCode = errors.New("código de recuperación inválido o expirado")
)

// logIgnoredError records a non-critical error without interrupting the main flow.
func logIgnoredError(action string, err error) {
	if err != nil {
		logger.Warn("ignored error", "action", action, "error", err)
	}
}
