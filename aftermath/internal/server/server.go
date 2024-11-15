package server

import (
	"aftermath/internal/database"
	"encoding/json"
	"net/http"
)

type Server struct {
	db *database.DB
}

func NewServer(db *database.DB) *Server {
	return &Server{db: db}
}

func (s *Server) Router() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/testing", s.handleQuery)
	return mux
}

func (s *Server) handleQuery(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode("Hello World")
}
