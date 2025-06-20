package middleware

import (
	path "github.com/godev90/netpath"
	"github.com/godev90/validator/errors"
)

func LocaleWrapper(next path.HandlerFunc) path.HandlerFunc {
	return func(ctx *path.Context) error {
		l := ctx.Query("lang")

		if l == string(errors.Bahasa) || l == string(errors.English) {
			ctx.UseLocale(errors.LanguageTag(l))
		}

		return next(ctx)
	}
}
