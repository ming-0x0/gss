package timezone

import "time"

const (
	UTC = "UTC"
	ICT = "ICT"
)

const (
	OffsetUTC = 0
	OffsetICT = 7 * 60 * 60
)

func setTimeZone(name string, offset int) {
	time.Local = time.FixedZone(name, offset)
}

func SetTimeZoneUTC() {
	setTimeZone(UTC, OffsetUTC)
}

// SetTimeZoneICT sets the local time zone to ICT (Indochina Time, UTC+7).
func SetTimeZoneICT() {
	setTimeZone(ICT, OffsetICT)
}
