package core

// Language identifies the UI language of the Eve client, inferred from the log header.
type Language string

const (
	LangEnglish  Language = "english"
	LangGerman   Language = "german"
	LangRussian  Language = "russian"
	LangFrench   Language = "french"
	LangJapanese Language = "japanese"
	LangChinese  Language = "chinese"
)

// Log type tag values as they appear inside the (type) field of each log line.
const (
	LogTypeCombat = "combat"
	LogTypeBounty = "bounty"
	LogTypeMining = "mining"
	LogTypeNone   = "None"
	LogTypeNotify = "notify"
)

// LocalePattern holds the language-specific strings used in Eve log headers
// and combat lines.
type LocalePattern struct {
	ListenerLabel    string // header line 3 prefix before character name
	SessionTimeLabel string // header line 4 prefix before the timestamp
	DirIn            string // direction keyword meaning "incoming" in combat lines
	DirOut           string // direction keyword meaning "outgoing" in combat lines
}

// Locales maps each supported Language to its header and combat patterns.
// Source: PELD logreader.py _logLanguageRegex.
var Locales = map[Language]LocalePattern{
	LangEnglish: {
		ListenerLabel:    "Listener: ",
		SessionTimeLabel: "Session Started: ",
		DirIn:            "from",
		DirOut:           "to",
	},
	LangGerman: {
		ListenerLabel:    "Empfänger: ",
		SessionTimeLabel: "Sitzung gestartet: ",
		DirIn:            "von",
		DirOut:           "nach",
	},
	LangRussian: {
		ListenerLabel:    "Слушатель: ",
		SessionTimeLabel: "Сеанс начат: ",
		DirIn:            "из",
		DirOut:           "на",
	},
	LangFrench: {
		ListenerLabel:    "Auditeur: ",
		SessionTimeLabel: "Session commencée: ",
		DirIn:            "de",
		DirOut:           "à",
	},
	LangJapanese: {
		ListenerLabel:    "傍聴者: ",
		SessionTimeLabel: "セッション開始: ",
		DirIn:            "攻撃者:",
		DirOut:           "対象:",
	},
	LangChinese: {
		ListenerLabel:    "收听者: ",
		SessionTimeLabel: "进程开始: ",
		DirIn:            "来自",
		DirOut:           "对",
	},
}
