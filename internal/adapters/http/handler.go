package httpadapter

import (
	"context"
	"errors"
	"net/http"

	"erp-schema/model"
	"erp/pkg/httpserver"
	configclient "erp/services/purchasing-service/internal/adapters/config"
	stockclient "erp/services/purchasing-service/internal/adapters/stock"
	cashclient "erp/services/purchasing-service/internal/adapters/cashflow"
	"erp/services/purchasing-service/internal/application"
	"erp/services/purchasing-service/internal/domain"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	svc *application.Service
}

func New(svc *application.Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Register(r *gin.Engine, jwt gin.HandlerFunc) {
	api := r.Group("/", jwt)
	api.GET("/quotes", h.listQuotes)
	api.POST("/quotes", h.createQuote)
	api.POST("/quotes/compare", h.compareQuotes)
	api.GET("/quotes/:id", h.getQuote)
	api.PUT("/quotes/:id", h.updateQuote)
	api.DELETE("/quotes/:id", h.deleteQuote)
	api.POST("/quotes/:id/convert", h.convertQuote)
	api.GET("/purchase-orders", h.listOrders)
	api.POST("/purchase-orders", h.createOrder)
	api.GET("/purchase-orders/:id", h.getOrder)
	api.PUT("/purchase-orders/:id", h.updateOrder)
	api.DELETE("/purchase-orders/:id", h.deleteOrder)
	api.POST("/purchase-orders/:id/receive", h.receive)
	api.POST("/purchase-orders/:id/confer", h.confer)
	api.POST("/purchase-orders/:id/cancel", h.cancel)
}

func (h *Handler) withAuth(c *gin.Context) context.Context {
	ctx := context.WithValue(c.Request.Context(), configclient.AuthHeaderKey, c.GetHeader("Authorization"))
	ctx = context.WithValue(ctx, stockclient.AuthHeaderKey, c.GetHeader("Authorization"))
	return context.WithValue(ctx, cashclient.AuthHeaderKey, c.GetHeader("Authorization"))
}

func (h *Handler) listQuotes(c *gin.Context) {
	out, err := h.svc.ListQuotes(c.Request.Context())
	if err != nil {
		httpserver.Error(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, fromDomainQuotes(out))
}

func (h *Handler) createQuote(c *gin.Context) {
	var in model.Quote
	if err := c.ShouldBindJSON(&in); err != nil {
		httpserver.Error(c, http.StatusBadRequest, err)
		return
	}
	out, err := h.svc.CreateQuote(h.withAuth(c), toDomainQuote(in))
	if err != nil {
		httpserver.Error(c, http.StatusBadRequest, err)
		return
	}
	c.JSON(http.StatusCreated, fromDomainQuote(out))
}

func (h *Handler) getQuote(c *gin.Context) {
	out, err := h.svc.GetQuote(c.Request.Context(), c.Param("id"))
	if err != nil {
		httpserver.Error(c, http.StatusNotFound, err)
		return
	}
	c.JSON(http.StatusOK, fromDomainQuote(out))
}

func (h *Handler) updateQuote(c *gin.Context) {
	var in model.Quote
	if err := c.ShouldBindJSON(&in); err != nil {
		httpserver.Error(c, http.StatusBadRequest, err)
		return
	}
	out, err := h.svc.UpdateQuote(h.withAuth(c), c.Param("id"), toDomainQuote(in))
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, domain.ErrNotFound) {
			status = http.StatusNotFound
		} else if errors.Is(err, domain.ErrInvalid) {
			status = http.StatusConflict
		}
		httpserver.Error(c, status, err)
		return
	}
	c.JSON(http.StatusOK, fromDomainQuote(out))
}

func (h *Handler) deleteQuote(c *gin.Context) {
	if err := h.svc.DeleteQuote(c.Request.Context(), c.Param("id")); err != nil {
		status := http.StatusNotFound
		if errors.Is(err, domain.ErrInvalid) {
			status = http.StatusConflict
		}
		httpserver.Error(c, status, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) convertQuote(c *gin.Context) {
	var in struct {
		PaymentMethodID string `json:"payment_method_id"`
		PaymentTermID   string `json:"payment_term_id"`
	}
	_ = c.ShouldBindJSON(&in)
	out, err := h.svc.ConvertQuote(h.withAuth(c), c.Param("id"), in.PaymentMethodID, in.PaymentTermID)
	if err != nil {
		status := http.StatusNotFound
		if errors.Is(err, domain.ErrInvalid) {
			status = http.StatusConflict
		}
		httpserver.Error(c, status, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *Handler) compareQuotes(c *gin.Context) {
	var in struct {
		IDs []string `json:"ids"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		httpserver.Error(c, http.StatusBadRequest, err)
		return
	}
	out, err := h.svc.CompareQuotes(c.Request.Context(), in.IDs)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, domain.ErrNotFound) {
			status = http.StatusNotFound
		}
		httpserver.Error(c, status, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) listOrders(c *gin.Context) {
	out, err := h.svc.ListOrders(c.Request.Context())
	if err != nil {
		httpserver.Error(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) createOrder(c *gin.Context) {
	var in domain.PurchaseOrder
	if err := c.ShouldBindJSON(&in); err != nil {
		httpserver.Error(c, http.StatusBadRequest, err)
		return
	}
	out, err := h.svc.CreateOrder(h.withAuth(c), in)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, domain.ErrQuoteRequired) || errors.Is(err, domain.ErrInvalid) {
			status = http.StatusConflict
		}
		httpserver.Error(c, status, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *Handler) getOrder(c *gin.Context) {
	out, err := h.svc.GetOrder(c.Request.Context(), c.Param("id"))
	if err != nil {
		httpserver.Error(c, http.StatusNotFound, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) updateOrder(c *gin.Context) {
	var in domain.PurchaseOrder
	if err := c.ShouldBindJSON(&in); err != nil {
		httpserver.Error(c, http.StatusBadRequest, err)
		return
	}
	out, err := h.svc.UpdateOrder(h.withAuth(c), c.Param("id"), in)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, domain.ErrNotFound) {
			status = http.StatusNotFound
		} else if errors.Is(err, domain.ErrInvalid) {
			status = http.StatusConflict
		}
		httpserver.Error(c, status, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) deleteOrder(c *gin.Context) {
	if err := h.svc.DeleteOrder(h.withAuth(c), c.Param("id")); err != nil {
		status := http.StatusNotFound
		if errors.Is(err, domain.ErrInvalid) {
			status = http.StatusConflict
		}
		httpserver.Error(c, status, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) receive(c *gin.Context) {
	var in struct {
		WarehouseID string `json:"warehouse_id"`
	}
	_ = c.ShouldBindJSON(&in)
	if err := h.svc.Receive(h.withAuth(c), c.Param("id"), in.WarehouseID); err != nil {
		status := http.StatusNotFound
		if errors.Is(err, domain.ErrInvalid) || errors.Is(err, domain.ErrWarehouseRequired) {
			status = http.StatusConflict
		}
		httpserver.Error(c, status, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) confer(c *gin.Context) {
	if err := h.svc.Confer(c.Request.Context(), c.Param("id")); err != nil {
		status := http.StatusNotFound
		if errors.Is(err, domain.ErrInvalid) {
			status = http.StatusConflict
		}
		httpserver.Error(c, status, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) cancel(c *gin.Context) {
	if err := h.svc.Cancel(c.Request.Context(), c.Param("id")); err != nil {
		status := http.StatusNotFound
		if errors.Is(err, domain.ErrInvalid) {
			status = http.StatusConflict
		}
		httpserver.Error(c, status, err)
		return
	}
	c.Status(http.StatusNoContent)
}
