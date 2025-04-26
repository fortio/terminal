package ansipixels

// Ansi codes.
const (
	Bold       = "\x1b[1m"
	Dim        = "\x1b[2m"
	Underlined = "\x1b[4m"
	Blink      = "\x1b[5m"
	Reverse    = "\x1b[7m"

	MoveLeft = "\033[1D"

	Reset = "\033[0m"
	// Foreground Colors.
	Black        = "\033[30m"
	Red          = "\033[31m"
	Green        = "\033[32m"
	Yellow       = "\033[33m"
	Blue         = "\033[34m"
	Purple       = "\033[35m"
	Cyan         = "\033[36m"
	Gray         = "\033[37m"
	DarkGray     = "\033[90m"
	BrightRed    = "\033[91m"
	BrightGreen  = "\033[92m"
	BrightYellow = "\033[93m"
	BrightBlue   = "\033[94m"
	BrightPurple = "\033[95m"
	BrightCyan   = "\033[96m"
	White        = "\033[97m"
	// Background Colors.
	BlackBG        = "\033[40m"
	RedBG          = "\033[41m"
	GreenBG        = "\033[42m"
	YellowBG       = "\033[43m"
	BlueBG         = "\033[44m"
	PurpleBG       = "\033[45m"
	CyanBG         = "\033[46m"
	GrayBG         = "\033[47m"
	DarkGrayBG     = "\033[100m"
	BrightRedBG    = "\033[101m"
	BrightGreenBG  = "\033[102m"
	BrightYellowBG = "\033[103m"
	BrightBlueBG   = "\033[104m"
	BrightPurpleBG = "\033[105m"
	BrightCyanBG   = "\033[106m"
	WhiteBG        = "\033[107m"

	// Inverse fg/bg colors.
	Inverse = "\033[7m"

	// Select colors from the 256 colors set that are missing from.
	Orange = "\033[38;5;214m"

	// Combo for RGB full pixel (used by fps).
	RedPixel   = Red + "█"
	GreenPixel = Green + "█"
	BluePixel  = Blue + "█"
	ResetClear = Reset + " "
)
