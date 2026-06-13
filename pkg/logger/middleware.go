package logger

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

var (
	handlerRegex = regexp.MustCompile(`/(.*?)\.\((.*?)\)\.(.*)`)
)

func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		loggerOptions := log.Logger.With()
		loggerOptions = loggerOptions.Caller()
		loggerOptions = loggerOptions.Str("handler", parseHandlerNameFuncOption(c.HandlerName()))
		loggerOptions = loggerOptions.Str("url", c.FullPath())
		loggerOptions = loggerOptions.Str("rawUrl", c.Request.URL.Path)
		loggerOptions = loggerOptions.Str("method", c.Request.Method)
		loggerOptions = loggerOptions.Int("pid", os.Getpid())
		requestID := c.GetHeader("X-Request-ID")
		if requestID != "" {
			loggerOptions = loggerOptions.Str("requestId", requestID)
		}
		logger := loggerOptions.Logger()

		ctx := logger.WithContext(c.Request.Context())
		c.Request = c.Request.Clone(ctx)
		logger.Info().Msg("Making Request")
		c.Next()
		logger = log.Ctx(c.Request.Context()).With().Int("status_code", c.Writer.Status()).Logger()
		logger.Info().Msg("Request Completed")
	}
}

// parseHandlerNameFuncOption removes the unnecessary information from the calling handler returning it in MQube friendly way.
// If includeFuncName is true, the function name will be included in the output.
// E.g. github.com/some-org/some-repo/some-package.(*Handler).SomeMethod-fm -> some-package.Handler.SomeMethod-fm or some-package.Handler
func parseHandlerNameFuncOption(fullHandlerName string) string {
	// Find the name of the handler struct. Contained in brackets ().
	matches := handlerRegex.FindStringSubmatch(fullHandlerName)
	if len(matches) != 4 {
		// Give up and return the full name
		return fullHandlerName
	}
	packagePath, structName, funcName := matches[1], matches[2], matches[3]

	// Remove the pointer (*) from the struct name if present
	structName = strings.Replace(structName, "*", "", 1)

	// Get the package name from the path
	packageName := filepath.Base(packagePath)

	parsedName := packageName + "." + structName
	parsedName += "." + funcName

	return parsedName
}
