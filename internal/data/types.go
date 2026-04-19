package data

import "encoding/json"

// CharField is one entry in Character.CharParam (named CharParam in TS).
type CharField struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	IsSilent bool   `json:"isSilent"`
	Sort     int    `json:"sort"`
	Category string `json:"category"`
}

// Character in a Plot.
type Character struct {
	Sort         int                  `json:"sort"`
	Color        *int                 `json:"color"`
	Priority     string               `json:"priority"`
	CharParam    map[string]CharField `json:"charParam"`
	CategoryList []json.RawMessage    `json:"categoryList"`
	TagList      string               `json:"tagList"`
	FolderPath   string               `json:"folderpath"`
}

// Name returns char_name.value or "Unnamed Character".
func (c Character) Name() string {
	if f, ok := c.CharParam["char_name"]; ok && f.Value != "" {
		return f.Value
	}
	return "Unnamed Character"
}

// Memo returns char_memo.value if present.
func (c Character) Memo() string {
	if f, ok := c.CharParam["char_memo"]; ok {
		return f.Value
	}
	return ""
}

// Field returns an arbitrary char_param value.
func (c Character) Field(name string) string {
	if f, ok := c.CharParam[name]; ok {
		return f.Value
	}
	return ""
}

// SequenceCard is a scene/card inside a SequenceUnit.
type SequenceCard struct {
	Idea                string            `json:"idea"`
	Description         string            `json:"description"`
	Place               string            `json:"place"`
	Timezone            string            `json:"timezone"`
	Memo                string            `json:"memo"`
	Color               *int              `json:"color"`
	Weather             string            `json:"weather"`
	CliffHanger         string            `json:"cliffHanger"`
	RelationIndexList   string            `json:"relationIndexList"`
	AreaMapIndexList    string            `json:"areaMapIndexList"`
	ImageList           string            `json:"imageList"`
	RelateAreaIndexList string            `json:"relateAreaIndexList"`
	Sort                int               `json:"sort"`
	SceneCardList       []json.RawMessage `json:"sceneCardList"`
	IsTextExpand        bool              `json:"isTextExpand"`
}

// SequenceUnit is a top-level act/section.
type SequenceUnit struct {
	Category         string         `json:"category"`
	Sort             int            `json:"sort"`
	SequenceCardList []SequenceCard `json:"sequenceCardList"`
	Title            string         `json:"title"`
	Message          string         `json:"message"`
	IsEdited         int            `json:"isEdited"`
	IsSilent         bool           `json:"isSilent"`
	IsTextExpand     bool           `json:"isTextExpand"`
}

// Relationship between two characters.
type Relationship struct {
	FromIndex   int    `json:"fromIndex"`
	ToIndex     int    `json:"toIndex"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Memo        string `json:"memo"`
	Color       *int   `json:"color"`
	Sort        int    `json:"sort"`
}

// EraEvent is a timeline event.
type EraEvent struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Memo        string `json:"memo"`
	EraIndex    int    `json:"eraIndex"`
	StartTime   string `json:"startTime"`
	EndTime     string `json:"endTime"`
	Sort        int    `json:"sort"`
	Color       *int   `json:"color"`
}

// Era of a plot's timeline.
type Era struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Memo        string `json:"memo"`
	StartTime   string `json:"startTime"`
	EndTime     string `json:"endTime"`
	Sort        int    `json:"sort"`
	Color       *int   `json:"color"`
}

// Area / location.
type Area struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Memo        string `json:"memo"`
	Sort        int    `json:"sort"`
	Color       *int   `json:"color"`
	Category    string `json:"category"`
}

// Plot is one story.
type Plot struct {
	Title                            string            `json:"title"`
	PlotType                         string            `json:"plotType"`
	ZoomLevel                        string            `json:"zoomLevel"`
	FolderPath                       string            `json:"folderPath"`
	Subtitle                         string            `json:"subtitle"`
	WritingStatus                    string            `json:"writingstatus"`
	Color                            *int              `json:"color"`
	TagList                          string            `json:"tagList"`
	SequenceUnitList                 []SequenceUnit    `json:"sequenceUnitList"`
	World                            json.RawMessage   `json:"world"`
	Logline                          json.RawMessage   `json:"logline"`
	CharList                         []Character       `json:"charList"`
	RelationShipList                 []Relationship    `json:"relationShipList"`
	RelationShipMapDetailList        []json.RawMessage `json:"relationShipMapDetailList"`
	RootFamilyList                   []json.RawMessage `json:"rootFamilyList"`
	TipFamilyList                    []json.RawMessage `json:"tipFamilyList"`
	ArrowTagColorList                []json.RawMessage `json:"arrowTagColorList"`
	GroupRelationList                []json.RawMessage `json:"groupRelationList"`
	GroupRelationArrowList           []json.RawMessage `json:"groupRelationArrowList"`
	EraList                          []Era             `json:"eraList"`
	EraEventList                     []EraEvent        `json:"eraEventList"`
	EraEventGroupList                []json.RawMessage `json:"eraEventGroupList"`
	CharFolderList                   []json.RawMessage `json:"charFolderList"`
	AreaList                         []Area            `json:"areaList"`
	AreaElementList                  []json.RawMessage `json:"areaElementList"`
	AreaMapRelationList              []json.RawMessage `json:"areaMapRelationList"`
	AreaMapDetailList                []json.RawMessage `json:"areaMapDetailList"`
	Sort                             int               `json:"sort"`
	UpdateTime                       int64             `json:"updateTime"`
	SortByUser                       int               `json:"sortByUser"`
	PinnedSort                       *int              `json:"pinnedSort"`
	DefaultTemplateCharacterIndex    int               `json:"defaultTemplateCharacterIndex"`
	DefaultTemplateAreaCountryIndex  int               `json:"defaultTemplateAreaCountryIndex"`
	DefaultTemplateAreaCityIndex     int               `json:"defaultTemplateAreaCityIndex"`
	DefaultTemplateAreaCommonIndex   int               `json:"defaultTemplateAreaCommonIndex"`
}

// Tags returns parsed plot tag list.
func (p Plot) Tags() []string {
	return parseTagListField(p.TagList)
}

// Folder entry.
type Folder struct {
	Type          string `json:"type"`
	Path          string `json:"path"`
	Sort          int    `json:"sort"`
	CreateTime    int64  `json:"createtime"`
	Color         *int   `json:"color"`
	IconImagePath string `json:"iconImagePath"`
	IconType      string `json:"iconType"`
	TagList       string `json:"taglist"`
}

// RawExport is the outer envelope (nested JSON stored as strings).
type RawExport struct {
	MemoList      string `json:"memoList"`
	TagColorMap   string `json:"tagColorMap"`
	PlotList      string `json:"plotList"`
	AllFolderList string `json:"allFolderList"`
}

// Export is the parsed, ready-to-use export.
type Export struct {
	MemoList      []json.RawMessage
	TagColorMap   map[string]json.RawMessage
	PlotList      []Plot
	AllFolderList []Folder
}

func parseTagListField(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil
	}
	return out
}
