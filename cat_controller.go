package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

type CatStruct struct {
	Fact   string `json:"fact"`
	Length int    `json:"length"`
}

type CatDataController struct {
	Controller
}

func FetchCatDataAPI(ctx context.Context) (CatStruct, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://catfact.ninja/fact", nil)
	if err != nil {
		return CatStruct{}, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return CatStruct{Fact: "Nenhum fato retornado", Length: -1}, err
	}
	defer resp.Body.Close()

	var catData CatStruct
	if err := json.NewDecoder(resp.Body).Decode(&catData); err != nil {
		return CatStruct{Fact: "Nenhum fato retornado", Length: -1}, err
	}

	time.Sleep(time.Millisecond * 100)

	return catData, nil
}

type CatDataResponse struct {
	value CatStruct
	err   error
}

func (cdc *CatDataController) GetCatData(cc context.Context, w http.ResponseWriter, r *http.Request) error {
	// pegando valor do context
	userID, ok := cc.Value("userID").(string)
	if !ok {
		return errors.New("userID not found in context")
	}

	ctx, cancel := context.WithCancel(cc)
	defer cancel() // garantindo que o ctx seja cancelado antes da função terminar para não vazar nenhum ctx
	respch := make(chan CatDataResponse)

	fmt.Println("UserID: ", userID)

	go func() {
		catData, err := FetchCatDataAPI(ctx)
		respch <- CatDataResponse{
			value: catData,
			err:   err,
		}
	}()

	fmt.Println(ctx)

	WriteJSONHelper(w, http.StatusOK, catData)
	return nil
}

func PassingCtxCatData(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	catCtr := &CatDataController{}
	userID := "2602"
	reqCtx := context.WithValue(ctx, "userID", userID)

	start := time.Now()
	err := catCtr.GetCatData(reqCtx, w, r)
	fmt.Println("Tempo de execução: ", time.Since(start))

	return err
}
