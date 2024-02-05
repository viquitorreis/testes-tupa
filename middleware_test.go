package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type TestController struct {
	*DefaultController
}

func (tc *TestController) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tc.router.ServeHTTP(w, r)
}

func MiddlewareSample(next APIFunc) APIFunc {

	return func(tc *TupaContext) error {
		smpMidd := "sampleMiddleware"
		reqCtx := context.WithValue(tc.request.Context(), "smpMidd", smpMidd)
		tc.request = tc.request.WithContext(reqCtx)

		log.SetFlags(log.LstdFlags | log.Lmicroseconds)
		fmt.Println("Middleware antes de chamar o handler")

		defer getCtxFromSampleMiddleware(tc)
		defer fmt.Println("SampleMiddleware depois de chamar o handler")
		return next(tc)
	}
}

func getCtxFromSampleMiddleware(tc *TupaContext) {
	ctxValue := tc.request.Context().Value("smpMidd").(string)

	fmt.Println(ctxValue)
}

func MiddlewareLoggingWithError(next APIFunc) APIFunc {
	return func(tc *TupaContext) error {

		start := time.Now()
		errMsg := errors.New("erro no middleware LoggingMiddlewareWithError")
		ctx := context.WithValue(tc.request.Context(), "smpErrorMidd", "sampleErrorMiddleware")
		tc.request = tc.request.WithContext(ctx)

		err := next(tc)
		log.SetFlags(log.LstdFlags | log.Lmicroseconds)
		log.Printf("Fim da request para endpoint teste: %s, duração: %v", tc.request.URL.Path, time.Since(start))

		if err != nil {
			log.Printf("erro ao chamar proximo middleware %s: %v", tc.request.URL.Path, err)
		}

		return errMsg
	}
}

func MiddlewareWithCtx(next APIFunc) APIFunc {

	return func(tc *TupaContext) error {
		smpMidd := "MiddlewareWithCtx"
		reqCtx := context.WithValue(tc.request.Context(), "withCtx", smpMidd)
		tc.request = tc.request.WithContext(reqCtx)

		log.SetFlags(log.LstdFlags | log.Lmicroseconds)

		return next(tc)
	}
}

func MiddlewareWithCtxChanMsg(next APIFunc, messages chan<- string) APIFunc {

	return func(tc *TupaContext) error {
		smpMidd := "MiddlewareWithCtx"
		reqCtx := context.WithValue(tc.request.Context(), "withCtx", smpMidd)
		tc.request = tc.request.WithContext(reqCtx)

		log.SetFlags(log.LstdFlags | log.Lmicroseconds)

		messages <- "MiddlewareWithCtx passou por aqui :)"

		return next(tc)
	}
}

func TestLoggingMiddlewareWithError(t *testing.T) {
	testMiddlewareWithError := MiddlewareChain{}
	testMiddlewareWithError.Use(MiddlewareLoggingWithError)

	errMsg := errors.New("erro no middleware LoggingMiddlewareWithError")
	testMiddlewareWithErrorController := &TestController{NewController()}
	testMiddlewareWithErrorController.RegisterRoutes("/error", []RouteInfo{
		{
			Method: MethodGet,
			Handler: func(tc *TupaContext) error {
				return tc.SendString(errMsg.Error())
			},
		},
	}, testMiddlewareWithError)

	server := httptest.NewServer(testMiddlewareWithErrorController)
	defer server.Close()

	resp, err := http.Get(server.URL + "/error")
	if err != nil {
		t.Fatalf("falhou ao fazer get reques: %v", err)
	}

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("esperava status code %d, recebeu %d", http.StatusInternalServerError, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("falhou ao ler response body: %v", err)
	}

	expectedErrorMessage := `{"Error":"erro no middleware LoggingMiddlewareWithError"}`
	if strings.TrimSuffix(string(body), "\n") != expectedErrorMessage {
		t.Errorf("esperou um response body %s, recebeu %s", expectedErrorMessage, body)
	}
}

func TestSampleMiddleware(t *testing.T) {
	t.Run("Testado TestSampleMiddleware", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		ctx := &TupaContext{
			request:  req,
			response: w,
		}

		// handler que não retorna erro
		handler := func(tc *TupaContext) error {
			// Chacando se o middleware tem valor de context
			if val := tc.request.Context().Value("someKey"); val != nil {
				t.Error("Valor de context esperado não era esperado")
			}
			return nil
		}

		err := MiddlewareSample(handler)(ctx)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		// Checando status code da response
		if status := w.Result().StatusCode; status != http.StatusOK {
			t.Errorf("handler retornou status code errado: recebeu %v queria %v", status, http.StatusOK)
		}
	})
}

func TestMiddlewareConcurrency(t *testing.T) {

	t.Run("Testando MiddlewareConcurrency", func(t *testing.T) {
		numGoroutines := 1000
		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		middleware := MiddlewareWithCtx

		handler := func(tc *TupaContext) error {
			ctxValue := tc.request.Context().Value("withCtx").(string)

			if ctxValue != "MiddlewareWithCtx" {
				t.Errorf("Esperava valor de context 'MiddlewareWithCtx', recebeu '%s'", ctxValue)
			}

			return nil
		}

		for i := 0; i < numGoroutines; i++ {
			go func() {
				// Criando nova request com context para cada goroutine
				req := httptest.NewRequest("GET", "/test", nil)
				w := httptest.NewRecorder()

				ctx := &TupaContext{
					request:  req,
					response: w,
				}

				// Chanmando o middleware com o handler e o contexto
				err := middleware(handler)(ctx)
				if err != nil {
					t.Errorf("erro nao esperado: %v", err)
				}

				wg.Done()
			}()
		}

		wg.Wait()
	})
}

func TestMiddlewareWithCtxAndChannels(t *testing.T) {
	numGoroutines := 1000
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	ctxValues := make(chan string, numGoroutines)

	middleware := MiddlewareWithCtx

	handler := func(tc *TupaContext) error {
		ctxValue := tc.request.Context().Value("withCtx").(string)

		ctxValues <- ctxValue

		return nil
	}

	for i := 0; i < numGoroutines; i++ {
		go func() {
			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()

			ctx := &TupaContext{
				request:  req,
				response: w,
			}

			err := middleware(handler)(ctx)
			if err != nil {
				t.Errorf("erro inesperado: %v", err)
			}

			wg.Done()
		}()
	}

	wg.Wait()
	close(ctxValues)

	for ctxValue := range ctxValues {
		if ctxValue != "MiddlewareWithCtx" {
			t.Errorf("Esperava 'MiddlewareWithCtx', recebeu '%s'", ctxValue)
		}
	}
}

func TestMiddlewareWithCtxAndChannelAndMsg(t *testing.T) {
	numGoroutines := 1000
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	messages := make(chan string, numGoroutines)

	middleware := func(next APIFunc) APIFunc {
		return MiddlewareWithCtxChanMsg(next, messages)
	}

	handler := func(tc *TupaContext) error {
		return nil
	}

	for i := 0; i < numGoroutines; i++ {
		go func() {
			// Criando nova req para cada goroutine
			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()

			ctx := &TupaContext{
				request:  req,
				response: w,
			}

			err := middleware(handler)(ctx)
			if err != nil {
				t.Errorf("erro inesperado: %v", err)
			}

			wg.Done()
		}()
	}

	wg.Wait()

	close(messages)

	for message := range messages {
		if message != "MiddlewareWithCtx passou por aqui :)" {
			t.Errorf("Expected message 'MiddlewareWithCtx passou por aqui :)', got '%s'", message)
		}
	}
}
