package lichess

type LightUser struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Title string `json:"title"`
	Bot   bool   `json:"bot"`
}

type Account struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Title    string `json:"title"`
	Online   bool   `json:"online"`
	Perfs    map[string]struct {
		Games  int `json:"games"`
		Rating int `json:"rating"`
	} `json:"perfs"`
}

type Variant struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

type ChallengeOptions struct {
	Rated     bool
	Limit     int
	Increment int
	Days      int
	Color     string
	Variant   string
	FEN       string
}

type Challenge struct {
	ID     string `json:"id"`
	URL    string `json:"url"`
	Status string `json:"status"`
	Color  string `json:"color"`
	Rated  bool   `json:"rated"`
}

type Event struct {
	Type      string    `json:"type"`
	Game      EventGame `json:"game"`
	Challenge Challenge `json:"challenge"`
}

type EventGame struct {
	ID       string    `json:"id"`
	GameID   string    `json:"gameId"`
	FullID   string    `json:"fullId"`
	Color    string    `json:"color"`
	Player   string    `json:"player"`
	FEN      string    `json:"fen"`
	LastMove string    `json:"lastMove"`
	Rated    bool      `json:"rated"`
	Speed    string    `json:"speed"`
	Variant  Variant   `json:"variant"`
	Opponent LightUser `json:"opponent"`
}

type BoardEvent struct {
	Type       string    `json:"type"`
	ID         string    `json:"id"`
	Rated      bool      `json:"rated"`
	Variant    Variant   `json:"variant"`
	Speed      string    `json:"speed"`
	InitialFEN string    `json:"initialFen"`
	White      GameUser  `json:"white"`
	Black      GameUser  `json:"black"`
	State      GameState `json:"state"`
	GameState
}

type GameUser struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Title   string `json:"title"`
	Rating  int    `json:"rating"`
	AILevel int    `json:"aiLevel"`
}

type GameState struct {
	Moves  string `json:"moves"`
	WTime  int    `json:"wtime"`
	BTime  int    `json:"btime"`
	WInc   int    `json:"winc"`
	BInc   int    `json:"binc"`
	WDraw  bool   `json:"wdraw"`
	BDraw  bool   `json:"bdraw"`
	Status string `json:"status"`
	Winner string `json:"winner"`
}
