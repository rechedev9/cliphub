package httpapi

const (
	mediaFile    = "a file or image a screen draws; tools return its URL"
	noScreen     = "no Studio screen calls it either"
	notAFeature  = "called by Studio itself or a plugin, not by a user"
	webEditionIO = "the web edition's own upload path; Studio imports through import_demo"
)

var routesWithoutMCPTool = map[Route]string{
	get("/api/jobs/{id}/renders/{variant}/publish"):                                      noScreen,
	get("/api/library/documents/{id}/content"):                                           mediaFile,
	get("/api/voice-plugin/live"):                                                        notAFeature,
	get("/api/full-demo/hud-themes/preview"):                                             mediaFile,
	get("/api/full-demo/overlay-designs/preview"):                                        mediaFile,
	get("/api/jobs/{id}/player-card/preview"):                                            mediaFile,
	get("/api/editor/assets/{id}/media"):                                                 mediaFile,
	get("/api/equipment/{name}/icon"):                                                    mediaFile,
	get("/api/faceit/flags/{code}"):                                                      mediaFile,
	get("/api/faceit/players/{playerID}/avatar"):                                         mediaFile,
	get("/api/maps/{map}/calibration"):                                                   mediaFile,
	get("/api/maps/{map}/shot"):                                                          mediaFile,
	get("/api/jobs/{id}/renders/{variant}/captions/{name}"):                              mediaFile,
	get("/api/jobs/{id}/renders/{variant}/covers/{name}"):                                mediaFile,
	get("/api/jobs/{id}/renders/{variant}/gallery"):                                      mediaFile,
	get("/api/jobs/{id}/renders/{variant}/revisions/{revision}/captions/{name}"):         mediaFile,
	get("/api/jobs/{id}/renders/{variant}/revisions/{revision}/covers/{name}"):           mediaFile,
	get("/api/jobs/{id}/renders/{variant}/revisions/{revision}/full-demo/{document}"):    mediaFile,
	get("/api/jobs/{id}/renders/{variant}/revisions/{revision}/gallery"):                 mediaFile,
	get("/api/jobs/{id}/renders/{variant}/revisions/{revision}/videos/{name}"):           mediaFile,
	get("/api/stream-jobs/{id}/renders/{variant}/gallery"):                               mediaFile,
	get("/api/stream-jobs/{id}/renders/{variant}/revisions/{revision}/delivery/{name}"):  mediaFile,
	get("/api/stream-jobs/{id}/renders/{variant}/revisions/{revision}/videos/{clip_id}"): mediaFile,
	get("/api/editor/projects"):                                                          noScreen,
	post("/api/editor/projects"):                                                         noScreen,
	get("/api/editor/projects/{id}"):                                                     noScreen,
	del("/api/editor/projects/{id}"):                                                     noScreen,
	get("/api/editor/projects/{id}/plan"):                                                noScreen,
	put("/api/editor/projects/{id}/plan"):                                                noScreen,
	post("/api/editor/projects/{id}/preview"):                                            noScreen,
	get("/api/editor/projects/{id}/render"):                                              noScreen,
	post("/api/editor/projects/{id}/render"):                                             noScreen,
	get("/api/editor/projects/{id}/render/cover"):                                        noScreen,
	get("/api/editor/projects/{id}/render/video"):                                        noScreen,
	get("/api/voice-profiles/{id}"):                                                      noScreen,
	put("/api/voice-profiles/{id}"):                                                      noScreen,
	del("/api/voice-profiles/{id}"):                                                      noScreen,
	get("/api/voice-profiles/{id}/audio"):                                                noScreen,
	get("/api/jobs/{id}/renders/{variant}/edit-document"):                                noScreen,
	get("/api/jobs/{id}/renders/{variant}/quality"):                                      noScreen,
	get("/api/loadouts"):                                                                 noScreen,
	get("/api/sync/changes"):                                                             notAFeature,
	post("/api/sync/changes"):                                                            notAFeature,
	post("/api/voice-plugin/gamestate"):                                                  notAFeature,
	post("/api/library/uploads"):                                                         webEditionIO,
}
