package provider

import (
	"errors"
	"net/http"

	tpuf "github.com/turbopuffer/turbopuffer-go"
)

func isNotFoundErr(err error) bool {
	var apiErr *tpuf.Error
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound
}
