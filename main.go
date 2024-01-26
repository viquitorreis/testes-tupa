package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rs/cors"
)

type Controller struct{}

type APIFunc func(context.Context, http.ResponseWriter, *http.Request) error

type APIServer struct {
	listenAddr string
	router     *mux.Router
}

type HTTPMethod string

type APIError struct {
	Error string
}

const (
	MethodGet     HTTPMethod = http.MethodGet
	MethodPost    HTTPMethod = http.MethodPost
	MethodPut     HTTPMethod = http.MethodPut
	MethodDelete  HTTPMethod = http.MethodDelete
	MethodPatch   HTTPMethod = http.MethodPatch
	MethodOptions HTTPMethod = http.MethodOptions
)

var AllowedMethods = map[HTTPMethod]bool{
	MethodGet:    true,
	MethodPost:   true,
	MethodPut:    true,
	MethodDelete: true,
	MethodPatch:  true,
}

func FmtBlue(s string) string {
	return "\033[1;34m" + s + "\033[0m"
}

func FmtRed(s string) string {
	return string("\033[1;31m") + s + string("\033[0m")
}

func FmtYellow(s string) string {
	return string("\033[1;33m ") + s + string("\033[0m")
}

func WriteJSONHelper(w http.ResponseWriter, status int, v any) error {
	w.Header().Set("Content-Type", "application/json")

	if w == nil {
		return errors.New("response writer passado está nulo")
	}

	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(v)
}

func (c *Controller) MakeHTTPHandlerFuncHelper(f APIFunc, httpMethod HTTPMethod) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if r.Method == string(httpMethod) {
			if err := f(ctx, w, r); err != nil {
				if err := WriteJSONHelper(w, http.StatusInternalServerError, APIError{Error: err.Error()}); err != nil {
					fmt.Println("Erro ao escrever resposta JSON:", err)
				}
			}
		} else {
			WriteJSONHelper(w, http.StatusMethodNotAllowed, APIError{Error: "Método HTTP não permitido"})
		}
	}
}

func (c *Controller) RegisterRoutes(router *mux.Router, route string, handlers map[HTTPMethod]APIFunc) {
	for method, handler := range handlers {
		if !AllowedMethods[method] {
			log.Fatal(fmt.Sprintf(FmtRed("Método HTTP não permitido: "), "%s\nVeja como criar um novo método na documentação", method))
		}
		router.HandleFunc(route, c.MakeHTTPHandlerFuncHelper(handler, method)).Methods(string(method))
	}
}

func (a *APIServer) SetDefaultRoute(handlers map[HTTPMethod]APIFunc) {
	defController := Controller{}
	for method, handler := range handlers {
		a.router.HandleFunc("/", defController.MakeHTTPHandlerFuncHelper(handler, method)).Methods(string(method))
	}
}

func WelcomeHandler(w http.ResponseWriter, r *http.Request) {
	WriteJSONHelper(w, http.StatusOK, "Seja bem vindo ao Tupã framework!")
}

func (a *APIServer) New() {

	if a.router.GetRoute("/") == nil {
		a.router.HandleFunc("/", WelcomeHandler).Methods(http.MethodGet)
	}

	fmt.Println(FmtBlue("Servidor iniciado na porta: " + a.listenAddr))

	routerHandler := cors.Default().Handler(a.router)

	if err := http.ListenAndServe(a.listenAddr, routerHandler); err != nil {
		log.Fatal(FmtRed("Erro ao iniciar servidor: "), err)
	}
}

func NewApiServer(listenAddr string) *APIServer {
	return &APIServer{
		listenAddr: listenAddr,
		router:     mux.NewRouter(),
		// store:      store,
	}
}

// TESTES HANDLERS

func HandleAPIEndpoint(w http.ResponseWriter, r *http.Request) error {
	fmt.Println("Endpoint da API")
	WriteJSONHelper(w, http.StatusOK, "Endpoint da API")
	return nil
}

func HandleAPIEndpoint2(w http.ResponseWriter, r *http.Request) error {
	fmt.Println("Hello world! Endpoint da API 2")
	WriteJSONHelper(w, http.StatusOK, "Endpoint da API")
	return nil
}

func HandlePOSTEndpoint(w http.ResponseWriter, r *http.Request) error {
	var testeString string

	if err := json.NewDecoder(r.Body).Decode(&testeString); err != nil {
		return err
	}

	fmt.Println(testeString)

	fmt.Println("Endpoint da API")
	WriteJSONHelper(w, http.StatusOK, testeString)
	return nil
}

func LinkMethodToHandler(method HTTPMethod, handler http.HandlerFunc) map[HTTPMethod]http.HandlerFunc {
	return map[HTTPMethod]http.HandlerFunc{
		method: handler,
	}
}

// user
func main() {
	server := NewApiServer(":6969")

	// catDataController := &CatDataController{}
	// catDataController.RegisterRoutes(server.router, "/catdata", map[HTTPMethod]APIFunc{
	// 	MethodGet: catDataController.GetCatData,
	// })

	catDataController2 := &CatDataController{}
	catDataController2.RegisterRoutes(server.router, "/catdata", map[HTTPMethod]APIFunc{
		MethodGet: PassingCtxCatData,
	})

	// server.SetDefaultRoute(map[HTTPMethod]APIFunc{
	// 	MethodGet:  HandleAPIEndpoint,
	// 	MethodPost: HandlePOSTEndpoint,
	// })

	// newController := Controller{}
	// newController.RegisterRoutes(server.router, "/api", map[HTTPMethod]APIFunc{
	// 	MethodGet:  HandleAPIEndpoint,
	// 	MethodPost: HandlePOSTEndpoint,
	// })

	// secondaryController := Controller{}
	// secondaryController.RegisterRoutes(server.router, "/api2", map[HTTPMethod]APIFunc{
	// 	MethodGet: HandleAPIEndpoint,
	// })

	server.New()

	// var userID string = "userID"
	// ctx := context.Background()

	// ctx = context.WithValue(ctx, userID, "1")

	// doFetchUserID(ctx)

}

///////// contexts
