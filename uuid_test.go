package rex

import (
	. "github.com/smartystreets/goconvey/convey"
	"testing"
)

var invalidUUIDs = []string{
	"00000000-0000-0000-0000-000000000000",
	"FFFFFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF",
	"ffffffff-ffff-ffff-ffff-ffffffffffff",
}

var validUUIDs = []string{
	"ED83F96D-D14E-4A81-8C7A-021DA055DEF5",
	"a4128ee3-5fd5-4359-bae4-6c03354f9a02",
	"28C1E1D8-7E76-4C57-A25A-4B8C81A3EB2A",
}

func TestUUIDHelpers(t *testing.T) {
	Convey("UUID Helpers", t, func() {
		Convey("FastValidateUUID", func() {
			for i := 0; i < len(validUUIDs); i++ {
				So(FastValidateUUID(validUUIDs[i]), ShouldBeTrue)
			}

			// FastValidate only checks length and structure
			for i := 0; i < len(invalidUUIDs); i++ {
				So(FastValidateUUID(invalidUUIDs[i]), ShouldBeTrue)
			}
		})

		Convey("ValidateUUID", func() {
			for i := 0; i < len(validUUIDs); i++ {
				So(ValidateUUID(validUUIDs[i]), ShouldBeTrue)
			}

			for i := 0; i < len(invalidUUIDs); i++ {
				So(ValidateUUID(invalidUUIDs[i]), ShouldBeFalse)
			}
		})
	})
}
