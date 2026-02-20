package logging

import (
	"bufio"
	"log"
	"net"
	"net/http"
)

// capture the status code from and http.ResponseWriter
// https://gist.github.com/Boerworz/b683e46ae0761056a636
type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lrw *loggingResponseWriter) Unwrap() http.ResponseWriter {
	return lrw.ResponseWriter
}

func (lrw *loggingResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	netConn, brw, err := http.NewResponseController(lrw.ResponseWriter).Hijack()
	return netConn, brw, err
}

func (lrw *loggingResponseWriter) WriteHeader(statusCode int) {
	lrw.statusCode = statusCode
	lrw.ResponseWriter.WriteHeader(statusCode)
}

func LogWare(next http.Handler, logger *log.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// logger.Printf("<-- %s %s", r.Method, r.URL.Path)
		lrw := loggingResponseWriter{w, 200}
		next.ServeHTTP(&lrw, r)
		logger.Printf("%d %s %s", lrw.statusCode, r.Method, r.URL.Path)
	})
}
