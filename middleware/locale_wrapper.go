package middleware

import (
	path "github.com/godev90/netpath"
	"github.com/godev90/validator/faults"
)

func LocaleWrapper(next path.HandlerFunc) path.HandlerFunc {
	return func(ctx *path.Context) error {
		l := ctx.Query("lang")

		if l == string(faults.Bahasa) || l == string(faults.English) {
			ctx.UseLocale(faults.LanguageTag(l))
		}

		return next(ctx)
	}
}
