// ClipHub's one player rating (`tacticalplan.Rating`): its name and the formula its (i) shows, shared by every view.

export const RATING_LABEL = 'Rating de ClipHub';

const RATING_FORMULA = '0,0073 KAST + 0,3591 KPR - 0,5329 DPR + 0,2372 impacto + 0,0032 ADR + 0,1587, con impacto = 2,13 KPR + 0,42 APR - 0,41';

export const RATING_FORMULA_TEXT = `${RATING_LABEL}: ${RATING_FORMULA}`;
