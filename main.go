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
	"reflect"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/cors"
)

func FmtBlue(s string) string {
	return "\033[1;34m" + s + "\033[0m"
}

func FmtRed(s string) string {
	return string("\033[1;31m") + s + string("\033[0m")
}

func FmtYellow(s string) string {
	return string("\033[1;33m ") + s + string("\033[0m")
}

// type Controller interface{
// 	RegisterRoutes(router *mux.Router, route string, handlers map[HTTPMethod]APIFunc)
// 	MakeHTTPHandlerFuncHelper(f APIFunc, httpMethod HTTPMethod) http.HandlerFunc
// }

type (
	Context interface {
		Request() *http.Request
		Response() http.ResponseWriter
		SendString(status int, s string) error
	}

	TupaContext struct {
		request  *http.Request
		response http.ResponseWriter
		context.Context
	}
)

type APIFunc func(*TupaContext) error

type APIServer struct {
	listenAddr string
	server     *http.Server
}

type HTTPMethod string

type APIError struct {
	Error string
}

type DefaultController struct {
	router *mux.Router
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

var (
	globalRouter     *mux.Router
	globalRouterOnce sync.Once
)

func (a *APIServer) New() {
	globalRouter = getGlobalRouter()

	if globalRouter.GetRoute("/") == nil {
		globalRouter.HandleFunc("/", WelcomeHandler).Methods(http.MethodGet)
	}

	routerHandler := cors.Default().Handler(globalRouter)

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
	<-signchan // vai esperar um comando que encerra o servidor

	ctx, shutdownRelease := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownRelease()

	if err := a.server.Shutdown(ctx); err != nil {
		log.Fatal(FmtRed("Erro ao desligar servidor: "), err)
	}

	fmt.Println(FmtYellow("Servidor encerrado na porta: " + a.listenAddr))
}

func NewApiServer(listenAddr string) *APIServer {
	return &APIServer{
		listenAddr: listenAddr,
	}
}

func (dc *DefaultController) SetDefaultRoute(handlers map[HTTPMethod]APIFunc) {
	for method, handler := range handlers {
		dc.router.HandleFunc("/", dc.MakeHTTPHandlerFuncHelper(handler, method)).Methods(string(method))
	}
}

func WelcomeHandler(w http.ResponseWriter, r *http.Request) {
	WriteJSONHelper(w, http.StatusOK, "Seja bem vindo ao Tupã framework!")
}

func (dc *DefaultController) RegisterRoutes(route string, handlers map[HTTPMethod]APIFunc) {
	// fmt.Println(handlers)
	for method, handler := range handlers {
		if !AllowedMethods[method] {
			log.Fatal(fmt.Sprintf(FmtRed("Método HTTP não permitido: "), "%s\nVeja como criar um novo método na documentação", method))
		}

		// handlerName := getFunctionName(handler)
		// fmt.Printf("Registering handler %s for %s\n", handlerName, method)

		dc.router.HandleFunc(route, dc.MakeHTTPHandlerFuncHelper(handler, method)).Methods(string(method))
	}
}

func WriteJSONHelper(w http.ResponseWriter, status int, v any) error {
	w.Header().Set("Content-Type", "application/json")

	if w == nil {
		return errors.New("Response writer passado está nulo")
	}

	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(v)
}

// func (dc *DefaultController) MakeHTTPHandlerFuncHelper(f APIFunc, httpMethod HTTPMethod) http.HandlerFunc {
// 	return func(w http.ResponseWriter, r *http.Request) {
// 		ctx := r.Context()
// 		if r.Method == string(httpMethod) {
// 			if err := f(ctx, w, r); err != nil {
// 				if err := WriteJSONHelper(w, http.StatusInternalServerError, APIError{Error: err.Error()}); err != nil {
// 					fmt.Println("Erro ao escrever resposta JSON:", err)
// 				}
// 			}
// 		} else {
// 			WriteJSONHelper(w, http.StatusMethodNotAllowed, APIError{Error: "Método HTTP não permitido"})
// 		}
// 	}
// }

func (dc *DefaultController) MakeHTTPHandlerFuncHelper(f APIFunc, httpMethod HTTPMethod) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := &TupaContext{
			request:  r,
			response: w,
		}
		if r.Method == string(httpMethod) {
			if err := f(ctx); err != nil {
				if err := WriteJSONHelper(w, http.StatusInternalServerError, APIError{Error: err.Error()}); err != nil {
					fmt.Println("Erro ao escrever resposta JSON:", err)
				}
			}
		} else {
			WriteJSONHelper(w, http.StatusMethodNotAllowed, APIError{Error: "Método HTTP não permitido"})
		}
	}
}

func NewController() *DefaultController {
	return &DefaultController{router: getGlobalRouter()} // cria sub rota para a rota
	// return &DefaultController{router: getGlobalRouter().PathPrefix(baseRoute).Subrouter()} // cria sub rota para a rota
}

func getGlobalRouter() *mux.Router {
	globalRouterOnce.Do(func() {
		globalRouter = mux.NewRouter()
	})
	return globalRouter
}

func (tc *TupaContext) Request() *http.Request {
	return tc.request
}

func (tc *TupaContext) Response() *http.ResponseWriter {
	return &tc.response
}

func (tc *TupaContext) SendString(status int, s string) error {
	// value := ctx.Value("value")

	value := tc.Context.Value("value")
	fmt.Printf("Value: %v\n", value)

	tc.response.WriteHeader(status)
	_, err := tc.response.Write([]byte(s))
	return err
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

func HandlePOSTEndpoint(tc *TupaContext) error {
	var testeString string

	if err := json.NewDecoder(tc.request.Body).Decode(&testeString); err != nil {
		return err
	}

	fmt.Println(testeString)
	time.Sleep(time.Second * 5)
	fmt.Println(FmtBlue("Depois de 5s do shutdown"))

	WriteJSONHelper(tc.response, http.StatusOK, testeString)
	return nil
}

// func LinkMethodToHandler(method HTTPMethod, handler http.HandlerFunc) map[HTTPMethod]http.HandlerFunc {
// 	return map[HTTPMethod]http.HandlerFunc{
// 		method: handler,
// 	}
// }

func getFunctionName(i interface{}) string {
	return runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
}

func handleSendString(tc *TupaContext) error {
	return tc.SendString(http.StatusOK, "Hello world")
}

// user
func main() {
	server := NewApiServer(":6969")
	// srv := tupa.NewApiServer(":6968")
	// testGracShutdown := tupa.Controller{}
	// testGracShutdown.RegisterRoutes(srv.Router, "/shutdown", map[tupa.HTTPMethod]tupa.APIFunc{
	// 	tupa.MethodPost: HandlePOSTEndpoint,
	// })
	// srv.New()

	// testGracefulShutdown := NewController()
	// testGracefulShutdown.RegisterRoutes("/shutdown", map[HTTPMethod]APIFunc{
	// 	MethodPost: HandlePOSTEndpoint,
	// 	MethodGet:  PassingCtxCatData,
	// })

	// catDataEndpoint := NewController()
	// catDataEndpoint.RegisterRoutes("/catdata", map[HTTPMethod]APIFunc{
	// 	MethodGet: PassingCtxCatData,
	// })

	// server.New()

	// testGracefulShutdown := tupa.NewController()
	// testGracefulShutdown.RegisterRoutes("/catdata", map[tupa.HTTPMethod]tupa.APIFunc{
	// 	tupa.MethodGet: PassingCtxCatData,
	// })

	testSendString := NewController()
	testSendString.RegisterRoutes("/ss", map[HTTPMethod]APIFunc{
		MethodGet: handleSendString,
	})

	testGracefulShutdown := NewController()
	testGracefulShutdown.RegisterRoutes("/shutdown", map[HTTPMethod]APIFunc{
		MethodPost: HandlePOSTEndpoint,
	})

	testCatDataController := NewController()
	testCatDataController.RegisterRoutes("/catdata", map[HTTPMethod]APIFunc{
		MethodGet: PassingCtxCatData,
	})

	server.New()

	// var userID string = "userID"
	// ctx := context.Background()

	// ctx = context.WithValue(ctx, userID, "1")

	// doFetchUserID(ctx)

}

///////// contexts
