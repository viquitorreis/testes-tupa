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
		SendString(s string) error
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
	listenAddr        string
	server            *http.Server
	globalMiddlewares MiddlewareChain
	router            *mux.Router
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

type RouteInfo struct {
	Path        string
	Method      HTTPMethod
	Handler     APIFunc
	Middlewares []MiddlewareFunc
}

func (a *APIServer) New() {
	if a.router.GetRoute("/") == nil {
		a.RegisterRoutes([]RouteInfo{
			{
				Path:    "/",
				Method:  MethodGet,
				Handler: WelcomeHandler,
			},
		})
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
	<-signchan // vai esperar um comando que encerra o servidor

	ctx, shutdownRelease := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownRelease()

	if err := a.server.Shutdown(ctx); err != nil {
		log.Fatal(FmtRed("Erro ao desligar servidor: "), err)
	}

	fmt.Println(FmtYellow("Servidor encerrado na porta: " + a.listenAddr))
}

func NewAPIServer(listenAddr string) *APIServer {
	router := mux.NewRouter()

	return &APIServer{
		listenAddr:        listenAddr,
		router:            router,
		globalMiddlewares: MiddlewareChain{},
	}
}

func WelcomeHandler(tc *TupaContext) error {
	WriteJSONHelper(tc.response, http.StatusOK, "Seja bem vindo ao Tupã framework!")
	return nil
}

func (a *APIServer) RegisterRoutes(routeInfos []RouteInfo) {
	for _, routeInfo := range routeInfos {
		if !AllowedMethods[routeInfo.Method] {
			log.Fatalf(fmt.Sprintf(FmtRed("Método HTTP não permitido: "), "%s\nVeja como criar um novo método na documentação", routeInfo.Method))
		}

		var allMiddlewares MiddlewareChain
		middlewaresGlobais := a.GetGlobalMiddlewares()

		allMiddlewares = append(allMiddlewares, middlewaresGlobais...)

		handler := a.MakeHTTPHandlerFuncHelper(routeInfo, allMiddlewares, a.globalMiddlewares)

		a.router.HandleFunc(routeInfo.Path, handler).Methods(string(routeInfo.Method))
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

func (a *APIServer) MakeHTTPHandlerFuncHelper(routeInfo RouteInfo, middlewares MiddlewareChain, globalMiddlewares MiddlewareChain) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := &TupaContext{
			request:  r,
			response: w,
		}

		// Combina middlewares globais com os especificos de rota
		allMiddlewares := MiddlewareChain{}
		allMiddlewares = append(allMiddlewares, a.GetGlobalMiddlewares()...)
		allMiddlewares = append(allMiddlewares, routeInfo.Middlewares...)

		doneCh := a.executeMiddlewaresAsync(ctx, allMiddlewares)
		errorsSlice := <-doneCh // espera até que algum valor seja recebido. Continua no primeiro erro recebido ( se houver ) ou se não houver nenhum erro

		if len(errorsSlice) > 0 {
			WriteJSONHelper(w, http.StatusInternalServerError, APIError{Error: errorsSlice[0].Error()})
			return
		}

		if r.Method == string(routeInfo.Method) {
			if err := routeInfo.Handler(ctx); err != nil {
				if err := WriteJSONHelper(w, http.StatusInternalServerError, APIError{Error: err.Error()}); err != nil {
					fmt.Println("Erro ao escrever resposta JSON:", err)
				}
			}
		} else {
			WriteJSONHelper(w, http.StatusMethodNotAllowed, APIError{Error: "Método HTTP não permitido"})
		}
	}
}

// func NewController() *APIServer {
// 	return &APIServer{router: getGlobalRouter()} // cria sub rota para a rota
// 	// return &DefaultController{router: getGlobalRouter().PathPrefix(baseRoute).Subrouter()} // cria sub rota para a rota
// }

// func getGlobalRouter() *mux.Router {
// 	globalRouterOnce.Do(func() {
// 		globalRouter = mux.NewRouter()
// 	})
// 	return globalRouter
// }

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
	return mux.Vars(tc.request)[param]
}

// TESTES HANDLERS

func HandleAPIEndpoint(tc *TupaContext) error {
	fmt.Println("Endpoint da API")
	WriteJSONHelper(tc.response, http.StatusOK, "Endpoint da API")
	return nil
}

func HandleAPIEndpoint3(tc *TupaContext) error {
	fmt.Println("Endpoint da HandleAPIEndpoint3")
	WriteJSONHelper(tc.response, http.StatusOK, "Endpoint da HandleAPIEndpoint3")
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

func MiddlewareAPIEndpoint(next APIFunc) APIFunc {
	return func(tc *TupaContext) error {
		fmt.Println("Middleware API Endpoint GLOBAL")
		return next(tc)
	}
}

func MiddlewareContrA(next APIFunc) APIFunc {
	return func(tc *TupaContext) error {
		fmt.Println("Middleware MiddlewareContrA")
		return next(tc)
	}
}

func MiddlewareContrB(next APIFunc) APIFunc {
	return func(tc *TupaContext) error {
		fmt.Println("Middleware MiddlewareContrB")
		return next(tc)
	}
}

func MiddlewareContrC(next APIFunc) APIFunc {
	return func(tc *TupaContext) error {
		fmt.Println("Middleware MiddlewareContrC")
		return next(tc)
	}
}

func MiddlewarePost(next APIFunc) APIFunc {
	return func(tc *TupaContext) error {
		fmt.Println("MiddlewarePost")
		return next(tc)
	}
}

func MiddlewareSampleErr(next APIFunc) APIFunc {
	return func(tc *TupaContext) error {
		fmt.Println("MiddlewareSampleErr")
		return errors.New("Erro no middleware MiddlewareSampleErr")
	}
}
func MiddlewareSampleErr2(next APIFunc) APIFunc {
	return func(tc *TupaContext) error {
		fmt.Println("MiddlewareSampleErr2")
		return errors.New("Erro no middleware MiddlewareSampleErr2")
	}
}

func MiddlewareGLOBAL(next APIFunc) APIFunc {
	return func(tc *TupaContext) error {
		fmt.Println("MiddlewareGLOBAL")
		return next(tc)
	}
}

// user
func main() {
	server := NewAPIServer(":6969")
	server.UseGlobalMiddleware(MiddlewareAPIEndpoint, MiddlewareGLOBAL)

	// middlewares := server.GetGlobalMiddlewares()
	// for _, middleware := range middlewares {
	// 	res := fmt.Sprintf("%+v\n", middleware)
	// 	fmt.Println("Hello world")
	// 	fmt.Println(res)
	// }

	// globalMiddlewares := MiddlewareChain{}
	// globalMiddlewares.Use(LoggingMiddlewareAfter)

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

	// testMiddlewareWithError := MiddlewareChain{}
	// testMiddlewareWithError.Use(LoggingMiddlewareAfter, HelloWorldMiddleware, LoggingMiddleware, LoggingMiddlewareWithErrorrr)

	// testMiddlewareWithErrorController := NewController()
	// testMiddlewareWithErrorController.RegisterRoutes("/error", []RouteInfo{
	// 	{
	// 		Method:      MethodGet,
	// 		Handler:     PassingCtxCatData,
	// 		Middlewares: []MiddlewareFunc{LoggingMiddlewareAfter, HelloWorldMiddleware, LoggingMiddleware},
	// 	},
	// 	{
	// 		Method:  MethodPost,
	// 		Handler: HandlePOSTEndpoint,
	// 	},
	// })

	// contr2 := NewController()

	// contr2.RegisterRoutes("/c2", []RouteInfo{
	// 	{
	// 		Method:      MethodGet,
	// 		Handler:     HandleAPIEndpoint,
	// 		Middlewares: []MiddlewareFunc{},
	// 	},
	// 	{
	// 		Method:      MethodPost,
	// 		Handler:     HandlePOSTEndpoint,
	// 		Middlewares: []MiddlewareFunc{MiddlewareSampleErr, MiddlewareSampleErr2, MiddlewarePost},
	// 	},
	// })

	routeInfo := RouteInfo{
		Path:        "/c2",
		Method:      MethodGet,
		Handler:     HandleAPIEndpoint,
		Middlewares: []MiddlewareFunc{MiddlewarePost},
	}

	server.RegisterRoutes([]RouteInfo{routeInfo})
	server.RegisterRoutes([]RouteInfo{
		{
			Path:        "/c3",
			Method:      MethodGet,
			Handler:     handleSendString,
			Middlewares: []MiddlewareFunc{HelloWorldMiddleware},
		},
		{
			Path:        "/c3",
			Method:      MethodPost,
			Handler:     HandlePOSTEndpoint,
			Middlewares: []MiddlewareFunc{MiddlewarePost},
		},
	})

	// server.RegisterRoutes(Routes)

	// ri := reflect.TypeOf(&RouteInfo{})
	// // ctrAVal := reflect.ValueOf(&CntrA{Routes: &Routes{}})
	// fmt.Println("--- Calling methods ---")
	// for i := 0; i < ri.NumMethod(); i++ {
	// 	method := ri.Method(i)
	// 	fmt.Println(method.Name)
	// 	// ctrAVal.MethodByName(method.Name).Call([]reflect.Value{})
	// 	// method.Func.Call([]reflect.Value{ctrAVal})
	// }
	// AddRoutes(nil, ContrARoutes, ContrBRoutes)
	// fmt.Println(GetRoutes())
	ExampleRouteManager()
	server.RegisterRoutes(GetRoutes())

	server.New()
}

///////// contexts
