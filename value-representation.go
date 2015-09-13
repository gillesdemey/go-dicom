package dicom

type VR int // Value Representation

const (
	AE = VR(iota) // Application Entity
	AS            // Age String
	AT            // Attribute Tag
	CS            // Code String
	DA            // Date
	DS            // Decimal String
	DT            // Date Time
	FL            // Floating Point Single
	FD            // Floating Point Double
	IS            // Integer String
	LO            // Long String
	LT            // Long Text
	OB            // Other Byte String
	OD            // Other Double String
	OF            // Other Float String
	OX            // Unknown
	OW            // Other Word String
	PN            // Person Name
	SH            // Short String
	SL            // Signed Long
	SQ            // Sequence of Items
	SS            // Signed Short
	ST            // Short Text
	TM            // Time
	UI            // Unique Identifier (UUID)
	UL            // Unsigned Long
	UN            // Unknown
	US            // Unsigned Short
	UT            // Unlimited Text
	NA            // Unknown
)

var m = map[string]VR{
	"AE": AE,
	"AS": AS,
	"AT": AT,
	"CS": CS,
	"DA": DA,
	"DS": DS,
	"DT": DT,
	"FL": FL,
	"FD": FD,
	"IS": IS,
	"LO": LO,
	"LT": LT,
	"OB": OB,
	"OD": OD,
	"OF": OF,
	"OW": OW,
	"PN": PN,
	"SH": SH,
	"SL": SL,
	"SQ": SQ,
	"SS": SS,
	"ST": ST,
	"TM": TM,
	"UI": UI,
	"UL": UL,
	"UN": UN,
	"US": US,
	"UT": UT,
}

func ParseVR(s string) VR {
	vr, found := m[s]
	if !found {
		return NA
	}
	return vr
}
