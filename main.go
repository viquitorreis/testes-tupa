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

type (
	Context interface {
		Request() *http.Request
		Response() http.ResponseWriter
		SendString(status int, s string) error
		Param(param string) string
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

// func (dc *DefaultController) SetDefaultRoute(handlers map[HTTPMethod]APIFunc) {
// 	for method, handler := range handlers {
// 		dc.router.HandleFunc("/", dc.MakeHTTPHandlerFuncHelper(handler, method)).Methods(string(method))
// 	}
// }

func WelcomeHandler(w http.ResponseWriter, r *http.Request) {
	WriteJSONHelper(w, http.StatusOK, "Seja bem vindo ao Tupã framework!")
}

type RouteInfo struct {
	Method  HTTPMethod
	Handler APIFunc
}

func (dc *DefaultController) RegisterRoutes(route string, routeInfos []RouteInfo, middlewares ...MiddlewareChain) {
	for _, routeInfo := range routeInfos {
		if !AllowedMethods[routeInfo.Method] {
			log.Fatalf(fmt.Sprintf(FmtRed("Método HTTP não permitido: "), "%s\nVeja como criar um novo método na documentação", routeInfo.Method))
		}

		handler := dc.MakeHTTPHandlerFuncHelper(routeInfo, middlewares...)

		dc.router.HandleFunc(route, handler).Methods(string(routeInfo.Method))
	}
}

func WriteJSONHelper(w http.ResponseWriter, status int, v any) error {
	if w == nil {
		return errors.New("Response writer passado está nulo")
	}

	w.Header().Set("Content-Type", "application/json")
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

func (dc *DefaultController) MakeHTTPHandlerFuncHelper(routeInfo RouteInfo, middlewares ...MiddlewareChain) http.HandlerFunc {
	fmt.Println("MakeHTTPHandlerFuncHelper")
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := &TupaContext{
			request:  r,
			response: w,
		}

		for _, middlewareChain := range middlewares {
			if err := middlewareChain.execute(ctx); err != nil {
				// Handle middleware error
				WriteJSONHelper(w, http.StatusInternalServerError, APIError{Error: err.Error()})
				return
			}
		}

		// for _, middlewareChain := range middlewares {
		// 	if err := middlewareChain.execute(ctx); err != nil {
		// 		// Handle middleware error
		// 		WriteJSONHelper(w, http.StatusInternalServerError, APIError{Error: err.Error()})
		// 		return
		// 	}
		// }

		if r.Method == string(routeInfo.Method) {
			if err := routeInfo.Handler(ctx); err != nil {
				if err := WriteJSONHelper(w, http.StatusInternalServerError, APIError{Error: err.Error()}); err != nil {
					fmt.Println("Erro ao escrever resposta JSON:", err)
				}
			}
		} else {
			WriteJSONHelper(w, http.StatusMethodNotAllowed, APIError{Error: "Método HTTP não permitido"})
		}
		// if err := routeInfo.Handler(ctx); err != nil {
		// 	// Handle handler error
		// 	WriteJSONHelper(w, http.StatusInternalServerError, APIError{Error: err.Error()})
		// 	return
		// }
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

func (tc *TupaContext) SendString(s string) error {
	_, err := tc.response.Write([]byte(s))
	return err
}

func (tc *TupaContext) Param(param string) string {
	// fmt.Println(mux.Vars(tc.request))
	// fmt.Println(tc.request)
	return mux.Vars(tc.request)[param]
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

func getFunctionName(i interface{}) string {
	return runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
}

func handleSendString(tc *TupaContext) error {
	return tc.SendString("Hello world")
}

func PrintRoutes(router *mux.Router) {
	router.Walk(func(route *mux.Route, router *mux.Router, ancestors []*mux.Route) error {
		pathTemplate, err := route.GetPathTemplate()
		if err == nil {
			methods, _ := route.GetMethods()
			fmt.Printf("\nRoute: %s\n Metodos: %s", pathTemplate, methods)
		}

		return nil
	})
}

// MIDDLEWARES EXEMPLOS

func LoggingMiddleware(next APIFunc) APIFunc {
	return func(tc *TupaContext) error {
		log.Println("Antes de chamar o handler")
		defer log.Println("Depois de chamar o handler")

		tupaContextWithValue := context.WithValue(tc.request.Context(), "ctxText", "2602")

		tc.request = tc.request.WithContext(tupaContextWithValue)
		// chamando o handler original
		return next(tc)
	}
}

func HelloWorldMiddleware(next APIFunc) APIFunc {
	return func(tc *TupaContext) error {
		log.Println("Hello world Antes de chamar o handler")
		defer log.Println("Hello world fim")

		// chamando o handler original
		return next(tc)
	}
}

func LoggingMiddlewareAfter(next APIFunc) APIFunc {
	fmt.Println("Middleware LoggingMiddlewareAfter")
	afterHandler := func(tc *TupaContext) error {
		defer log.Println("Depois de chamar o handler 2")
		return next(tc)
	}

	return afterHandler
}

func LoggingMiddlewareWithErrorrr(next APIFunc) APIFunc {
	return func(tc *TupaContext) error {

		// errMsg := errors.New("erro no middleware LoggingMiddlewareWithError")
		// err := WriteMiddErrorJSONHelper(tc, errMsg)
		// tc.response.WriteHeader(http.StatusInternalServerError)

		// escrevendo o erro no body da req
		// tc.response.Write([]byte(`{"Error":"` + "houve um erro" + `"}`))

		ctx := context.WithValue(tc.request.Context(), "smpErrorMidd", "sampleErrorMiddleware")
		tc.request = tc.request.WithContext(ctx)

		return errors.New("erro no middleware LoggingMiddlewareWithError")
	}
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

	// testSendString := NewController()
	// testSendString.RegisterRoutes("/ss", map[HTTPMethod]APIFunc{
	// 	MethodGet: LoggingMiddleware(handleSendString),
	// })

	// testGracefulShutdown := NewController()
	// testGracefulShutdown.RegisterRoutes("/shutdown", map[HTTPMethod]APIFunc{
	// 	MethodPost: HandlePOSTEndpoint,
	// })

	// testCatDataController := NewController()
	// testCatDataController.RegisterRoutes("/catdata", map[HTTPMethod]APIFunc{
	// 	MethodGet: PassingCtxCatData,
	// })

	// testParamChain := MiddlewareChain{}
	// testParamChain.Use(LoggingMiddleware, LoggingMiddlewareAfter)

	// testGetParam := NewController()
	// testGetParam.RegisterRoutes("/param", map[HTTPMethod]APIFunc{
	// 	MethodGet: handleSendString,
	// })
	// testGetParam.RegisterRoutes("/param/{id}", map[HTTPMethod]APIFunc{
	// 	MethodGet: func(tc *TupaContext) error {
	// 		fmt.Println("Chamou o handler")
	// 		ctxVal := tc.request.Context().Value("ctxText")
	// 		fmt.Println("Valor do context da req", ctxVal)
	// 		tc.SendString(http.StatusOK, "HELLO WORLD!"+tc.Param("id"))
	// 		return nil
	// 	},
	// }, testParamChain)

	// middChain := MiddlewareChain{}
	// middChain.Use(LoggingMiddleware, HelloWorldMiddleware)
	// middChainController := NewController()
	// middChainController.RegisterRoutes("/midd", map[HTTPMethod]APIFunc{
	// 	MethodGet: PassingCtxCatData,
	// }, middChain)

	testMiddlewareWithError := MiddlewareChain{}
	testMiddlewareWithError.Use(LoggingMiddlewareAfter, HelloWorldMiddleware)
	testMiddlewareWithErrorController := NewController()
	testMiddlewareWithErrorController.RegisterRoutes("/error", []RouteInfo{
		{
			Method:  MethodGet,
			Handler: PassingCtxCatData,
		},
		{
			Method:  MethodPost,
			Handler: HandlePOSTEndpoint,
		},
	}, testMiddlewareWithError)

	server.New()
}

///////// contexts
