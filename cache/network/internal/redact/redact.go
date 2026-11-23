// Package redact keeps presigned URLs out of logs and error text. Every URL the
// cache API hands out for an archive is presigned: whoever reads it can fetch
// the object until it expires. So a URL is replaced whole, with no guess about
// which part of it is the secret.
package redact

import (
	"fmt"
	"net/url"
	"regexp"

	"github.com/bitrise-io/go-utils/v2/log"
)

const placeholder = "<redacted URL>"

// Ends at whitespace or a quote, and leaves the punctuation of the sentence
// around the URL in place, as in "GET <url>: retrying".
var urlInText = regexp.MustCompile(`https?://[^\s"'<>]*[^\s"'<>.,:;)]`)

// Error returns the error of a failed request without the request's URL.
// errors.Is and errors.As find the same causes as before, and an error that
// names no URL is returned as it is.
func Error(err error) error {
	if err == nil {
		return nil
	}
	// What http.Client returns: its message is `Get "<url>": <cause>`.
	if e, ok := err.(*url.Error); ok {
		return &url.Error{Op: e.Op, URL: placeholder, Err: Error(e.Err)}
	}
	// A wrapper that prints the URL itself, like retryablehttp's "giving up" error.
	if msg := text(err.Error()); msg != err.Error() {
		return &redacted{msg: msg, err: err}
	}
	return err
}

// Logger returns a logger that redacts every URL in what it prints. It is for
// the libraries that log on this package's behalf (got, retryablehttp), where
// the call sites can't be changed.
func Logger(l log.Logger) log.Logger {
	return logger{l}
}

type redacted struct {
	msg string
	err error
}

func (e *redacted) Error() string { return e.msg }
func (e *redacted) Unwrap() error { return e.err }

func text(s string) string {
	return urlInText.ReplaceAllString(s, placeholder)
}

type logger struct{ log.Logger }

// Infof implements log.Logger.
func (l logger) Infof(format string, v ...any) { l.Logger.Infof("%s", sprintf(format, v)) }

// Warnf implements log.Logger.
func (l logger) Warnf(format string, v ...any) { l.Logger.Warnf("%s", sprintf(format, v)) }

// Printf implements log.Logger.
func (l logger) Printf(format string, v ...any) { l.Logger.Printf("%s", sprintf(format, v)) }

// Donef implements log.Logger.
func (l logger) Donef(format string, v ...any) { l.Logger.Donef("%s", sprintf(format, v)) }

// Debugf implements log.Logger.
func (l logger) Debugf(format string, v ...any) { l.Logger.Debugf("%s", sprintf(format, v)) }

// Errorf implements log.Logger.
func (l logger) Errorf(format string, v ...any) { l.Logger.Errorf("%s", sprintf(format, v)) }

// TInfof implements log.Logger.
func (l logger) TInfof(format string, v ...any) { l.Logger.TInfof("%s", sprintf(format, v)) }

// TWarnf implements log.Logger.
func (l logger) TWarnf(format string, v ...any) { l.Logger.TWarnf("%s", sprintf(format, v)) }

// TPrintf implements log.Logger.
func (l logger) TPrintf(format string, v ...any) { l.Logger.TPrintf("%s", sprintf(format, v)) }

// TDonef implements log.Logger.
func (l logger) TDonef(format string, v ...any) { l.Logger.TDonef("%s", sprintf(format, v)) }

// TDebugf implements log.Logger.
func (l logger) TDebugf(format string, v ...any) { l.Logger.TDebugf("%s", sprintf(format, v)) }

// TErrorf implements log.Logger.
func (l logger) TErrorf(format string, v ...any) { l.Logger.TErrorf("%s", sprintf(format, v)) }

func sprintf(format string, v []any) string {
	return text(fmt.Sprintf(format, v...))
}
