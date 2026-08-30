package reporting

import (
	"sort"
	"strings"

	"fiberpulse.dev/agent/internal/localization"
)

var reportLanguages = map[string]int{"fr": 0, "de": 1, "es": 2, "pt-BR": 3, "it": 4, "hi": 5}

// The PDF renderer routes every visible string through this dictionary. Plan,
// provider and subscriber values are deliberately left unchanged.
var reportTranslations = map[string][6]string{
	"COMPLAINT DOSSIER":                                {"DOSSIER DE RÉCLAMATION", "BESCHWERDEDOSSIER", "EXPEDIENTE DE RECLAMACIÓN", "DOSSIÊ DE RECLAMAÇÃO", "DOSSIER DI RECLAMO", "शिकायत डॉसियर"},
	"PERFORMANCE OVERVIEW":                             {"VUE D’ENSEMBLE DES PERFORMANCES", "LEISTUNGSÜBERSICHT", "RESUMEN DE RENDIMIENTO", "VISÃO GERAL DO DESEMPENHO", "PANORAMICA DELLE PRESTAZIONI", "प्रदर्शन सारांश"},
	"Connection complaint dossier":                     {"Dossier de réclamation de connexion", "Beschwerdedossier zum Anschluss", "Expediente de reclamación de conexión", "Dossiê de reclamação da conexão", "Dossier di reclamo sulla connessione", "कनेक्शन शिकायत डॉसियर"},
	"Seven-day evidence package for technical support": {"Dossier de preuves sur sept jours pour l’assistance technique", "Siebentägiges Nachweispaket für den Support", "Pruebas de siete días para soporte técnico", "Evidências de sete dias para o suporte técnico", "Prove di sette giorni per l’assistenza tecnica", "तकनीकी सहायता के लिए सात दिन के प्रमाण"},
	"COLLECTION IN PROGRESS":                           {"COLLECTE EN COURS", "SAMMLUNG LÄUFT", "RECOPILACIÓN EN CURSO", "COLETA EM ANDAMENTO", "RACCOLTA IN CORSO", "संग्रह जारी है"},
	"READY TO SEND":                                    {"PRÊT À ENVOYER", "VERSANDBEREIT", "LISTO PARA ENVIAR", "PRONTO PARA ENVIAR", "PRONTO PER L’INVIO", "भेजने के लिए तैयार"},
	"Generated ":                                       {"Généré ", "Erstellt ", "Generado ", "Gerado ", "Generato ", "तैयार किया "},
	"Period ":                                          {"Période ", "Zeitraum ", "Periodo ", "Período ", "Periodo ", "अवधि "},
	"Evidence:":                                        {"Preuves :", "Nachweise:", "Pruebas:", "Evidências:", "Prove:", "प्रमाण:"},
	"tests":                                            {"tests", "Tests", "pruebas", "testes", "test", "परीक्षण"},
	"days":                                             {"jours", "Tage", "días", "dias", "giorni", "दिन"},
	"SUBSCRIBER AND SERVICE":                           {"ABONNÉ ET SERVICE", "KUNDE UND DIENST", "ABONADO Y SERVICIO", "ASSINANTE E SERVIÇO", "ABBONATO E SERVIZIO", "ग्राहक और सेवा"},
	"Details supplied by the account holder":           {"Informations fournies par le titulaire du compte", "Angaben des Kontoinhabers", "Datos facilitados por el titular", "Dados fornecidos pelo titular", "Dati forniti dall’intestatario", "खाताधारक द्वारा दी गई जानकारी"},
	"Account holder":                                   {"Titulaire du compte", "Kontoinhaber", "Titular", "Titular", "Intestatario", "खाताधारक"},
	"Account number":                                   {"Numéro de compte", "Kundennummer", "Número de cuenta", "Número da conta", "Numero cliente", "खाता संख्या"},
	"Service address":                                  {"Adresse du service", "Anschlussadresse", "Dirección del servicio", "Endereço do serviço", "Indirizzo del servizio", "सेवा का पता"},
	"Contact":                                          {"Contact", "Kontakt", "Contacto", "Contato", "Contatto", "संपर्क"},
	"SUBSCRIBED OFFER AND OBSERVED PERFORMANCE":    {"OFFRE SOUSCRITE ET PERFORMANCES OBSERVÉES", "TARIF UND GEMESSENE LEISTUNG", "OFERTA CONTRATADA Y RENDIMIENTO", "PLANO CONTRATADO E DESEMPENHO", "OFFERTA E PRESTAZIONI OSSERVATE", "सदस्यता प्लान और मापा प्रदर्शन"},
	"Provider / offer":                             {"Fournisseur / offre", "Anbieter / Tarif", "Proveedor / oferta", "Provedor / plano", "Provider / offerta", "प्रदाता / प्लान"},
	"Advertised":                                   {"Annoncé", "Beworben", "Anunciado", "Anunciado", "Pubblicizzato", "विज्ञापित"},
	"Seven-day median":                             {"Médiane sur sept jours", "Sieben-Tage-Median", "Mediana de siete días", "Mediana de sete dias", "Mediana di sette giorni", "सात दिन का मध्य"},
	"Test conditions":                              {"Conditions de test", "Testbedingungen", "Condiciones de prueba", "Condições do teste", "Condizioni di test", "परीक्षण की स्थितियाँ"},
	"INSTALLATION PROFILE":                         {"PROFIL DE L’INSTALLATION", "INSTALLATIONSPROFIL", "PERFIL DE INSTALACIÓN", "PERFIL DA INSTALAÇÃO", "PROFILO DELL’INSTALLAZIONE", "इंस्टॉलेशन प्रोफ़ाइल"},
	"PROVIDER SUPPORT":                             {"ASSISTANCE DU FOURNISSEUR", "ANBIETER-SUPPORT", "SOPORTE DEL PROVEEDOR", "SUPORTE DO PROVEDOR", "ASSISTENZA DEL PROVIDER", "प्रदाता सहायता"},
	"Internet performance report":                  {"Rapport de performance Internet", "Internet-Leistungsbericht", "Informe de rendimiento de Internet", "Relatório de desempenho da Internet", "Rapporto sulle prestazioni Internet", "इंटरनेट प्रदर्शन रिपोर्ट"},
	"Private local report":                         {"Rapport local privé", "Privater lokaler Bericht", "Informe local privado", "Relatório local privado", "Rapporto locale privato", "निजी स्थानीय रिपोर्ट"},
	"COMPLETE TESTS":                               {"TESTS TERMINÉS", "VOLLSTÄNDIGE TESTS", "PRUEBAS COMPLETAS", "TESTES COMPLETOS", "TEST COMPLETI", "पूरे परीक्षण"},
	"MEDIAN DOWNLOAD":                              {"TÉLÉCHARGEMENT MÉDIAN", "MEDIAN DOWNLOAD", "DESCARGA MEDIANA", "DOWNLOAD MEDIANO", "DOWNLOAD MEDIANO", "मध्य डाउनलोड"},
	"MEDIAN UPLOAD":                                {"ENVOI MÉDIAN", "MEDIAN UPLOAD", "SUBIDA MEDIANA", "UPLOAD MEDIANO", "UPLOAD MEDIANO", "मध्य अपलोड"},
	"MEDIAN LATENCY":                               {"LATENCE MÉDIANE", "MEDIANE LATENZ", "LATENCIA MEDIANA", "LATÊNCIA MEDIANA", "LATENZA MEDIANA", "मध्य विलंबता"},
	"LATEST MEASUREMENT":                           {"DERNIÈRE MESURE", "NEUESTE MESSUNG", "ÚLTIMA MEDICIÓN", "MEDIÇÃO MAIS RECENTE", "ULTIMA MISURAZIONE", "नवीनतम माप"},
	"The most recent complete test in this report": {"Le test complet le plus récent de ce rapport", "Der neueste vollständige Test in diesem Bericht", "La prueba completa más reciente del informe", "O teste completo mais recente deste relatório", "Il test completo più recente del rapporto", "इस रिपोर्ट का सबसे नया पूरा परीक्षण"},
	"qualified":                                    {"qualifiés", "qualifiziert", "válidas", "qualificados", "qualificati", "योग्य"},
	"DOWNLOAD":                                     {"TÉLÉCHARGEMENT", "DOWNLOAD", "DESCARGA", "DOWNLOAD", "DOWNLOAD", "डाउनलोड"},
	"UPLOAD":                                       {"ENVOI", "UPLOAD", "SUBIDA", "UPLOAD", "UPLOAD", "अपलोड"},
	"MIN LATENCY":                                  {"LATENCE MIN.", "MIN. LATENZ", "LATENCIA MÍN.", "LATÊNCIA MÍN.", "LATENZA MIN.", "न्यूनतम विलंबता"},
	"CONFIDENCE":                                   {"CONFIANCE", "VERTRAUEN", "CONFIANZA", "CONFIANÇA", "AFFIDABILITÀ", "विश्वसनीयता"},
	"PLAN CHECK":                                   {"VÉRIFICATION DE L’OFFRE", "TARIFPRÜFUNG", "COMPROBACIÓN DEL PLAN", "VERIFICAÇÃO DO PLANO", "VERIFICA DEL PIANO", "प्लान जाँच"},
	"Measured performance compared with the selected Internet offer":                                                                        {"Performances mesurées comparées à l’offre Internet sélectionnée", "Messleistung im Vergleich zum gewählten Internettarif", "Rendimiento medido frente al plan seleccionado", "Desempenho medido comparado ao plano selecionado", "Prestazioni misurate rispetto al piano selezionato", "चुने गए इंटरनेट प्लान से मापे प्रदर्शन की तुलना"},
	"No Internet plan was selected when this report was generated. Select your provider and offer in FiberPulse to include a plan verdict.": {"Aucune offre Internet n’était sélectionnée lors de la création du rapport. Sélectionnez votre fournisseur et votre offre dans FiberPulse pour inclure un verdict.", "Beim Erstellen des Berichts war kein Internettarif ausgewählt. Wählen Sie Anbieter und Tarif in FiberPulse für eine Bewertung.", "No había ningún plan seleccionado al crear el informe. Selecciona proveedor y oferta en FiberPulse para incluir una evaluación.", "Nenhum plano estava selecionado ao gerar o relatório. Selecione provedor e plano no FiberPulse para incluir uma avaliação.", "Nessun piano era selezionato durante la creazione. Scegli provider e offerta in FiberPulse per includere una valutazione.", "रिपोर्ट बनाते समय कोई इंटरनेट प्लान नहीं चुना गया था। मूल्यांकन जोड़ने के लिए FiberPulse में प्रदाता और प्लान चुनें।"},
	"HOW TO USE THIS REPORT": {"COMMENT UTILISER CE RAPPORT", "SO VERWENDEN SIE DIESEN BERICHT", "CÓMO USAR ESTE INFORME", "COMO USAR ESTE RELATÓRIO", "COME USARE QUESTO RAPPORTO", "इस रिपोर्ट का उपयोग कैसे करें"},
	"Evidence to support diagnosis and a conversation with your provider": {"Éléments pour le diagnostic et l’échange avec votre fournisseur", "Nachweise für Diagnose und Gespräch mit dem Anbieter", "Pruebas para el diagnóstico y la conversación con el proveedor", "Evidências para diagnóstico e conversa com o provedor", "Prove per la diagnosi e il confronto con il provider", "निदान और प्रदाता से बातचीत के लिए प्रमाण"},
	"FiberPulse reports application-level NDT7 measurements to a nearby neutral M-Lab server. Tests are refused while a VPN route is detected. Repeated low results are useful evidence of delivered performance, but one test alone does not prove ISP responsibility or physical line capacity. For a complaint, run tests at different times, connect by Ethernet directly to the provider router where possible, pause heavy traffic and attach this PDF plus the CSV export.": {"FiberPulse relève des mesures NDT7 au niveau applicatif vers un serveur M-Lab neutre proche. Les tests sont refusés si un VPN est détecté. Des résultats faibles répétés documentent la performance fournie, mais un test seul ne prouve ni la responsabilité du fournisseur ni la capacité physique de la ligne. Pour une réclamation, testez à différents horaires, utilisez si possible Ethernet directement sur le routeur du fournisseur, interrompez le trafic important et joignez ce PDF avec l’export CSV.", "FiberPulse erfasst NDT7-Messungen auf Anwendungsebene zu einem nahen neutralen M-Lab-Server. Bei erkanntem VPN werden Tests abgelehnt. Wiederholt niedrige Werte belegen die gelieferte Leistung, ein einzelner Test beweist jedoch weder die Verantwortung des Anbieters noch die physische Leitungskapazität. Testen Sie zu verschiedenen Zeiten möglichst direkt per Ethernet, pausieren Sie starken Datenverkehr und fügen Sie PDF und CSV bei.", "FiberPulse realiza mediciones NDT7 a nivel de aplicación hacia un servidor neutral M-Lab cercano. Se rechazan las pruebas si se detecta una VPN. Los resultados bajos repetidos documentan el rendimiento, pero una sola prueba no demuestra la responsabilidad del proveedor ni la capacidad física de la línea. Para reclamar, prueba a distintas horas, usa Ethernet directo cuando sea posible, pausa el tráfico intenso y adjunta este PDF y el CSV.", "O FiberPulse faz medições NDT7 no nível do aplicativo em um servidor neutro M-Lab próximo. Testes são recusados quando uma VPN é detectada. Resultados baixos repetidos documentam o desempenho, mas um teste isolado não prova a responsabilidade do provedor nem a capacidade física da linha. Para reclamar, teste em horários diferentes, use Ethernet direto quando possível, pause tráfego intenso e anexe este PDF e o CSV.", "FiberPulse esegue misure NDT7 a livello applicativo verso un server M-Lab neutrale vicino. I test vengono rifiutati se viene rilevata una VPN. Risultati bassi ripetuti documentano le prestazioni, ma un solo test non prova la responsabilità del provider né la capacità fisica della linea. Per un reclamo, esegui test in orari diversi, usa Ethernet diretto quando possibile, sospendi il traffico intenso e allega PDF e CSV.", "FiberPulse पास के निष्पक्ष M-Lab सर्वर पर ऐप-स्तर के NDT7 माप दर्ज करता है। VPN मिलने पर परीक्षण रोके जाते हैं। बार-बार कम परिणाम मिली हुई गति के उपयोगी प्रमाण हैं, लेकिन एक परीक्षण अकेले प्रदाता की जिम्मेदारी या लाइन की भौतिक क्षमता सिद्ध नहीं करता। शिकायत के लिए अलग-अलग समय पर परीक्षण करें, जहाँ संभव हो प्रदाता राउटर से सीधे Ethernet जोड़ें, भारी ट्रैफ़िक रोकें और इस PDF के साथ CSV निर्यात संलग्न करें।"},
	"MEASUREMENT HISTORY":                                {"HISTORIQUE DES MESURES", "MESSVERLAUF", "HISTORIAL DE MEDICIONES", "HISTÓRICO DE MEDIÇÕES", "CRONOLOGIA DELLE MISURAZIONI", "माप इतिहास"},
	"Recent observations in reverse chronological order": {"Observations récentes, de la plus récente à la plus ancienne", "Neueste Beobachtungen in umgekehrter Reihenfolge", "Observaciones recientes en orden cronológico inverso", "Observações recentes em ordem cronológica inversa", "Osservazioni recenti in ordine cronologico inverso", "नवीनतम माप पहले"},
	"DATE UTC": {"DATE UTC", "DATUM UTC", "FECHA UTC", "DATA UTC", "DATA UTC", "UTC तारीख"},
	"STATUS":   {"ÉTAT", "STATUS", "ESTADO", "STATUS", "STATO", "स्थिति"},
	"DOWN":     {"DESC.", "DOWN", "BAJADA", "DOWN", "DOWN", "डाउन"},
	"UP":       {"ENV.", "UP", "SUBIDA", "UP", "UP", "अप"},
	"CONF.":    {"CONF.", "VERTR.", "CONF.", "CONF.", "AFFID.", "विश्व."},
	"COMPLETE": {"TERMINÉ", "VOLLSTÄNDIG", "COMPLETA", "COMPLETO", "COMPLETO", "पूर्ण"},
	"FiberPulse - local-first Internet performance evidence": {"FiberPulse — preuves de performance Internet conservées localement", "FiberPulse – lokal gespeicherte Internet-Leistungsnachweise", "FiberPulse — pruebas locales de rendimiento de Internet", "FiberPulse — evidências locais de desempenho da Internet", "FiberPulse — prove locali delle prestazioni Internet", "FiberPulse — स्थानीय इंटरनेट प्रदर्शन प्रमाण"},
	"Page ":         {"Page ", "Seite ", "Página ", "Página ", "Pagina ", "पृष्ठ "},
	"Not provided":  {"Non renseigné", "Nicht angegeben", "No indicado", "Não informado", "Non indicato", "नहीं दिया गया"},
	"Not selected":  {"Non sélectionné", "Nicht ausgewählt", "No seleccionado", "Não selecionado", "Non selezionato", "नहीं चुना गया"},
	"Not available": {"Indisponible", "Nicht verfügbar", "No disponible", "Indisponível", "Non disponibile", "उपलब्ध नहीं"},
}

func localizeReportText(value, language string) string {
	index, ok := reportLanguages[localization.Normalize(language)]
	if !ok || value == "" {
		return value
	}
	keys := make([]string, 0, len(reportTranslations))
	for key := range reportTranslations {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return len(keys[i]) > len(keys[j]) })
	pairs := make([]string, 0, len(keys)*2)
	for _, key := range keys {
		pairs = append(pairs, key, reportTranslations[key][index])
	}
	return strings.NewReplacer(pairs...).Replace(value)
}

func localizedCSVHeader(language string) []string {
	headers := map[string][]string{
		"en":    {"started_at_utc", "provider", "server", "download_bps", "upload_bps", "min_rtt_us", "status", "confidence_score", "confidence_level", "public_eligible", "reason_codes"},
		"fr":    {"début_utc", "fournisseur", "serveur", "téléchargement_bps", "envoi_bps", "rtt_min_us", "état", "score_confiance", "niveau_confiance", "publication_admissible", "codes_motif"},
		"de":    {"start_utc", "anbieter", "server", "download_bps", "upload_bps", "min_rtt_us", "status", "vertrauenswert", "vertrauensniveau", "öffentlich_zulässig", "grundcodes"},
		"es":    {"inicio_utc", "proveedor", "servidor", "descarga_bps", "subida_bps", "rtt_mín_us", "estado", "puntuación_confianza", "nivel_confianza", "publicación_admisible", "códigos_motivo"},
		"pt-BR": {"início_utc", "provedor", "servidor", "download_bps", "upload_bps", "rtt_mín_us", "status", "pontuação_confiança", "nível_confiança", "publicação_elegível", "códigos_motivo"},
		"it":    {"inizio_utc", "provider", "server", "download_bps", "upload_bps", "rtt_min_us", "stato", "punteggio_affidabilità", "livello_affidabilità", "pubblicazione_idonea", "codici_motivo"},
		"hi":    {"आरंभ_utc", "प्रदाता", "सर्वर", "डाउनलोड_bps", "अपलोड_bps", "न्यूनतम_rtt_us", "स्थिति", "विश्वसनीयता_स्कोर", "विश्वसनीयता_स्तर", "सार्वजनिक_योग्य", "कारण_कोड"},
	}
	language = localization.Normalize(language)
	if header, ok := headers[language]; ok {
		return header
	}
	return headers["en"]
}
