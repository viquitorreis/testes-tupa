package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/cors"
)

type Controller struct{}

type APIFunc func(context.Context, http.ResponseWriter, *http.Request) error

type APIServer struct {
	listenAddr string
	router     *mux.Router

	//
	server *http.Server
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

	routerHandler := cors.Default().Handler(a.router)

	a.server = &http.Server{
		Addr:    a.listenAddr,
		Handler: routerHandler,
	}

	fmt.Println(FmtBlue("Servidor iniciado na porta: " + a.listenAddr))

	go func() {
		if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(FmtRed("Erro ao iniciar servidor: "), err)
		}
		log.Println(FmtYellow("Servidor parou de receber novas conexões"))
	}()

	signchan := make(chan os.Signal, 1)
	signal.Notify(signchan, syscall.SIGINT, syscall.SIGTERM)
	<-signchan

	ctx, shutdownRelease := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownRelease()

	if err := a.server.Shutdown(ctx); err != nil {
		log.Fatal(FmtRed("Erro ao desligar servidor: "), err)
	}

	///////////////// NAO TA CHEGANDO AQ
	fmt.Println(FmtYellow("Servidor encerrado na porta: " + a.listenAddr))

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

func HandlePOSTEndpoint(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	var testeString string

	if err := json.NewDecoder(r.Body).Decode(&testeString); err != nil {
		return err
	}

	fmt.Println(testeString)
	time.Sleep(time.Second * 5)
	fmt.Println(FmtBlue("Depois de 5s do shutdown"))

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

	// catDataController2 := &CatDataController{}
	// catDataController2.RegisterRoutes(server.router, "/catdata", map[HTTPMethod]APIFunc{
	// 	MethodGet: PassingCtxCatData,
	// })

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

	testGracefulShutdown := Controller{}
	testGracefulShutdown.RegisterRoutes(server.router, "/shutdown", map[HTTPMethod]APIFunc{
		MethodPost: HandlePOSTEndpoint,
	})

	server.New()

	// var userID string = "userID"
	// ctx := context.Background()

	// ctx = context.WithValue(ctx, userID, "1")

	// doFetchUserID(ctx)

}

///////// contexts
