package api

import (
	"github.com/mooncorn/gshub/api/internal/database"
	"github.com/mooncorn/gshub/api/internal/services/k8s"
)

type ServerHandler struct {
	db        *database.DB
	k8sClient *k8s.Client
}

func NewServerHandler(db *database.DB, k8sClient *k8s.Client) *ServerHandler {
	return &ServerHandler{
		db:        db,
		k8sClient: k8sClient,
	}
}
