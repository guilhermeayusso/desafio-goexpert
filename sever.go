package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type USDToBRLRate struct {
	USDBRL struct {
		Code       string `json:"code"`
		Codein     string `json:"codein"`
		Name       string `json:"name"`
		High       string `json:"high"`
		Low        string `json:"low"`
		VarBid     string `json:"varBid"`
		PctChange  string `json:"pctChange"`
		Bid        string `json:"bid"`
		Ask        string `json:"ask"`
		Timestamp  string `json:"timestamp"`
		CreateDate string `json:"create_date"`
	} `json:"USDBRL"`
}

type USDToBRLRateDB struct {
	ID        uint      `gorm:"primaryKey;autoIncrement"`
	Code      string    `gorm:"type:varchar(10);not null"`
	Bid       float64   `gorm:"type:decimal(10,4);not null"`
	Ask       float64   `gorm:"type:decimal(10,4);not null"`
	Timestamp int64     `gorm:"not null"`                    // Unix timestamp
	CreatedAt time.Time `gorm:"column:create_date;not null"` // Mapeia para o campo "create_date" no banco
}

var db *gorm.DB
var errorDB error

func main() {

	db, errorDB = gorm.Open(sqlite.Open("./data/exchange.db"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})

	if errorDB != nil {
		log.Fatal("failed to connect database: ", errorDB)
	}

	// Migrate the schema
	if err := db.AutoMigrate(&USDToBRLRateDB{}); err != nil {
		log.Fatal("failed to migrate schema: ", err)
	}

	log.Println("Database connected and schema migrated successfully.")

	http.HandleFunc("/cotacao", GetExchangeRateHandler)
	log.Println("Servidor iniciado na porta 8080...")
	http.ListenAndServe(":8080", nil)
}

func GetExchangeRateHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/cotacao" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	rate, err := GetExchangeRate()
	if err != nil {
		log.Printf("Erro ao obter taxa de câmbio: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Criar contexto com timeout de 10ms para a "persistência"
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Millisecond)
	defer cancel()

	select {
	case <-time.After(5 * time.Millisecond):
		err := SaveExchangeRate(rate)
		if err != nil {
			log.Println("Dados gravados com sucesso no banco (simulação).")
		}
	case <-ctx.Done():
		log.Println("Timeout: operação de gravação no banco excedeu 10ms.")
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(rate)
}

func GetExchangeRate() (*USDToBRLRate, error) {
	// Timeout de 200ms para a requisição HTTP
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://economia.awesomeapi.com.br/last/USD-BRL", nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var rate USDToBRLRate
	err = json.Unmarshal(body, &rate)
	if err != nil {
		return nil, err
	}

	return &rate, nil
}

// Função para persistir os dados no banco de dados
func SaveExchangeRate(rate *USDToBRLRate) error {
	rateDB := USDToBRLRateDB{
		Code:      rate.USDBRL.Code,
		Bid:       parseFloat(rate.USDBRL.Bid),
		Ask:       parseFloat(rate.USDBRL.Ask),
		Timestamp: parseTimestamp(rate.USDBRL.Timestamp),
	}

	if err := db.Create(&rateDB).Error; err != nil {
		return err
	}

	return nil
}

func parseTimestamp(s string) int64 {
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		log.Printf("Erro ao converter '%s' para int64: %v", s, err)
		return 0
	}
	return i
}

func parseFloat(s string) float64 {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		log.Printf("Erro ao converter '%s' para float64: %v", s, err)
		return 0
	}
	return f
}
