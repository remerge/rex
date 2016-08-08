package rex

import (
	"fmt"
	"os"

	. "github.com/smartystreets/goconvey/convey"
	"testing"
)

func TestUtil(t *testing.T) {
	Convey("util", t, func() {
		Convey("PathExist", func() {
			Convey("false if path missing", func() {
				fileName := "/" + GenRandFilename(32)

				exists, existErr := PathExists(fileName)
				So(existErr, ShouldBeNil)
				So(exists, ShouldEqual, false)
			})

			Convey("true if path exists", func() {
				wd, wdErr := os.Getwd()
				So(wdErr, ShouldBeNil)

				fileName := fmt.Sprintf("%s/%s", wd, GenRandFilename(32))
				f, createErr := os.Create(fileName)
				So(createErr, ShouldBeNil)

				f.Close()

				exists, existErr := PathExists(fileName)
				So(existErr, ShouldBeNil)
				So(exists, ShouldEqual, true)

				rmErr := os.Remove(fileName)
				So(rmErr, ShouldBeNil)
			})
		})

		Convey("GenRandFilename", func() {
			Convey("generates a string of the specified length", func() {
				two := GenRandFilename(2)
				five := GenRandFilename(5)
				big := GenRandFilename(128)

				So(len(two), ShouldEqual, 2)
				So(len(five), ShouldEqual, 5)
				So(len(big), ShouldEqual, 128)
			})

			// Don't bet your life on this one
			Convey("generates psuedo-random strings", func() {
				str1 := GenRandFilename(128)
				str2 := GenRandFilename(128)

				So(str1, ShouldNotEqual, str2)
			})
		})
	})
}
