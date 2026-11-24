package customhud

import "github.com/rechedev9/cliphub/internal/overlaylang"

type verticalText struct {
	round, freeze, planted, defusing, ended string
}

func verticalTexts(language overlaylang.Language) verticalText {
	if language == overlaylang.English {
		return verticalText{round: "ROUND %d", freeze: "FREEZE TIME", planted: "BOMB PLANTED", defusing: "DEFUSING", ended: "ROUND END"}
	}
	return verticalText{round: "RONDA %d", freeze: "COMPRAS", planted: "BOMBA PLANTADA", defusing: "DESACTIVANDO", ended: "FIN DE RONDA"}
}

type VerticalScript struct {
	Theme    Theme
	Cues     []VerticalCue
	Language overlaylang.Language
}
