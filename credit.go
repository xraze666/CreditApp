package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
)

type LoanData struct {
	FullPrice      float64
	DownPayment    float64
	MonthsToPay    int
	MonthlyPayment float64
	Success        bool
	Error          string
}

type JSONResponse struct {
	Success        bool    `json:"success"`
	Message        string  `json:"message"`
	MonthlyPayment float64 `json:"monthly_payment,omitempty"`
	ErrorMessage   string  `json:"error,omitempty"`
	ErrorCode      int     `json:"error_code,omitempty"`
}

func errorMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("Panic: %v", err)
				w.Header().Set("Content-Type", "application/json")
				statusCode := http.StatusInternalServerError
				if strings.Contains(fmt.Sprint(err), "502") {
					statusCode = http.StatusBadGateway
				}
				w.WriteHeader(statusCode)
				json.NewEncoder(w).Encode(JSONResponse{
					Success:      false,
					ErrorMessage: "Sorry bro, nothing to see here, just 500...",
					ErrorCode:    statusCode,
				})
			}
		}()

		srw := &statusResponseWriter{ResponseWriter: w}
		next.ServeHTTP(srw, r)

		if srw.status == http.StatusInternalServerError && !srw.wroteJSON {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(JSONResponse{
				Success:      false,
				ErrorMessage: "Sorry bro... the site is still being built..",
				ErrorCode:    500,
			})
		}
	}
}

type statusResponseWriter struct {
	http.ResponseWriter
	status    int
	wroteJSON bool
}

func (w *statusResponseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusResponseWriter) Write(b []byte) (int, error) {
	if w.Header().Get("Content-Type") == "application/json" {
		var js json.RawMessage
		if json.Unmarshal(b, &js) == nil {
			w.wroteJSON = true
		}
	}
	return w.ResponseWriter.Write(b)
}

func main() {
	filePath := "index.html"

	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		fmt.Println("Ошибка при чтении файла:", err)
		return
	}

	tmpl := template.Must(template.New("loan").Parse(string(content)))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		tmpl.Execute(w, nil)
	})

	http.HandleFunc("/calculate", errorMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(JSONResponse{
				Success:      false,
				ErrorMessage: "Method not allowed",
				ErrorCode:    405,
			})
			return
		}

		fullPrice, err1 := strconv.ParseFloat(r.FormValue("fullPrice"), 64)
		downPayment, err2 := strconv.ParseFloat(r.FormValue("downPayment"), 64)
		monthsToPay, err3 := strconv.Atoi(r.FormValue("monthsToPay"))

		var response JSONResponse

		switch {
		case err1 != nil || err2 != nil || err3 != nil:
			response = JSONResponse{
				Success:      false,
				ErrorMessage: "Пожалуйста, введите корректные числовые значения",
				ErrorCode:    400,
			}
		case fullPrice <= 0:
			response = JSONResponse{
				Success:      false,
				ErrorMessage: "Стоимость должна быть больше 0",
				ErrorCode:    400,
			}
		case downPayment >= fullPrice:
			response = JSONResponse{
				Success:      false,
				ErrorMessage: "Первоначальный взнос не может быть больше полной стоимости",
				ErrorCode:    400,
			}
		case downPayment < 0:
			response = JSONResponse{
				Success:      false,
				ErrorMessage: "Первоначальный взнос не может быть отрицательным",
				ErrorCode:    400,
			}
		case monthsToPay <= 0:
			response = JSONResponse{
				Success:      false,
				ErrorMessage: "Срок кредита должен быть больше 0 месяцев",
				ErrorCode:    400,
			}
		case monthsToPay > 360:
			response = JSONResponse{
				Success:      false,
				ErrorMessage: "Максимальный срок кредита - 360 месяцев",
				ErrorCode:    400,
			}
		default:
			loanAmount := fullPrice - downPayment
			monthlyPayment := loanAmount / float64(monthsToPay)
			response = JSONResponse{
				Success:        true,
				Message:        "Заявка успешно создана",
				MonthlyPayment: monthlyPayment,
			}
		}

		if !response.Success {
			w.WriteHeader(response.ErrorCode)
		}
		json.NewEncoder(w).Encode(response)
	}))

	fmt.Println("Server is running on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
