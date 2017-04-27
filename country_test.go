package rex

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestCountryNormalizedAlpha2(t *testing.T) {
	Convey("normalises 2-digit upper case code", t, func() {
		So(CountryNormalizedAlpha2("DE"), ShouldEqual, "de")
	})

	Convey("normalises 3-digit lower case code", t, func() {
		So(CountryNormalizedAlpha2("gbr"), ShouldEqual, "gb")
	})

	Convey("normalises ea non-standard names", t, func() {
		So(CountryNormalizedAlpha2("United States"), ShouldEqual, "us")
	})

	Convey("returns xx for empty code", t, func() {
		So(CountryNormalizedAlpha2(""), ShouldEqual, "xx")
	})

	Convey("returns xx for invalid country", t, func() {
		So(CountryNormalizedAlpha2("Arstotzka"), ShouldEqual, "xx")
	})
}
