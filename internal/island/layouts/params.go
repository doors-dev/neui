package layouts

type ColumnParams struct {
	_    struct{} `additionalProperties:"false"`
	Gap  string   `json:"gap" enum:"sm,md,lg" default:"md" description:"space between children"`
	Fill int      `json:"fill,omitempty" minimum:"1" maximum:"6" description:"only when this column is directly inside a row: share of the line's leftover space, split by weight"`
}

type RowParams struct {
	_       struct{} `additionalProperties:"false"`
	Gap     string   `json:"gap" enum:"sm,md,lg" default:"md" description:"space between children"`
	Align   string   `json:"align" enum:"start,center,end" default:"start" description:"vertical alignment of children within a line; center lines up a row of controls of different heights"`
	Justify string   `json:"justify" enum:"start,end,between" default:"start" description:"horizontal distribution of children along a line"`
}

type GridParams struct {
	_    struct{} `additionalProperties:"false"`
	Gap  string   `json:"gap" enum:"sm,md,lg" default:"md" description:"space between cells"`
	Item string   `json:"item" enum:"sm,md,lg" default:"md" description:"minimum cell width"`
}
