package complaint

import (
	"strings"

	"fiberpulse.dev/agent/internal/localization"
	"fiberpulse.dev/agent/internal/plan"
)

// BuildDraftLocalized keeps the provider-ready structure identical in every
// language while leaving subscriber-entered values and official plan names
// untouched.
func BuildDraftLocalized(profile Profile, offer *plan.Offer, contact SupportContact, assessment Assessment, language string) Draft {
	draft := BuildDraft(profile, offer, contact, assessment)
	pairs := draftTranslationPairs(localization.Normalize(language))
	if len(pairs) == 0 {
		return draft
	}
	replacer := strings.NewReplacer(pairs...)
	draft.Subject = replacer.Replace(draft.Subject)
	draft.Body = replacer.Replace(draft.Body)
	draft.CallScript = replacer.Replace(draft.CallScript)
	draft.Warning = replacer.Replace(draft.Warning)
	return draft
}

func draftTranslationPairs(language string) []string {
	switch language {
	case "fr":
		return []string{
			"Request for technical investigation", "Demande d’investigation technique", "Dear ", "Bonjour, service assistance ", " Support,", ",",
			"I am requesting a technical investigation into the performance of my Internet connection.", "Je demande une investigation technique sur les performances de ma connexion Internet.",
			"Subscriber details", "Coordonnées de l’abonné", "Account holder:", "Titulaire du compte :", "Account number:", "Numéro de compte :", "Service address:", "Adresse du service :", "Contact email:", "E-mail :", "Contact phone:", "Téléphone :",
			"Subscribed service", "Service souscrit", "Provider:", "Fournisseur :", "Offer:", "Offre :", "Advertised download: up to", "Débit descendant annoncé : jusqu’à",
			"FiberPulse monitoring summary", "Résumé de la surveillance FiberPulse", "Monitoring period:", "Période :", "Qualified measurements:", "Mesures qualifiées :", "across", "sur", "day(s)", "jour(s)", "Median download:", "Téléchargement médian :", "of the advertised speed", "du débit annoncé", "Median upload:", "Envoi médian :", "Median minimum latency:", "Latence minimale médiane :", "Ethernet tests:", "Tests Ethernet :", "Wi-Fi tests:", "Tests Wi-Fi :",
			"Installation and test conditions", "Installation et conditions de test", "Provider modem / ONT:", "Modem / ONT du fournisseur :", "Provider router:", "Routeur du fournisseur :", "Additional router:", "Routeur supplémentaire :", "Mesh system:", "Système maillé :", "Main test connection:", "Connexion principale du test :", "Network layout:", "Topologie réseau :", "Typical connected devices:", "Appareils habituellement connectés :", "Additional notes:", "Notes supplémentaires :",
			"The attached FiberPulse PDF contains the individual measurements and methodology notes. The tests are application-level M-Lab NDT7 measurements and are provided as diagnostic evidence, not as proof of physical line capacity.", "Le PDF FiberPulse joint contient les mesures individuelles et les notes méthodologiques. Ces tests M-Lab NDT7 au niveau applicatif constituent des éléments de diagnostic et non une preuve de la capacité physique de la ligne.",
			"Please review the line provisioning, ONT or modem status, optical signal where applicable, router configuration, and possible congestion. Please create a technical support ticket and reply with the ticket reference and your findings.", "Merci de vérifier le provisionnement, l’état de l’ONT ou du modem, le signal optique le cas échéant, la configuration du routeur et une éventuelle congestion. Merci de créer un ticket technique et de répondre avec sa référence et vos conclusions.",
			"Kind regards,", "Cordialement,", "Not provided", "Non renseigné", "None", "Aucune", "No", "Non", "Yes - model not provided", "Oui — modèle non renseigné", "Yes - ", "Oui — ",
			"Hello, I am ", "Bonjour, je suis ", ". My account number is ", ". Mon numéro de compte est ", " and my plan is ", " et mon offre est ", ", advertised at up to ", ", annoncée jusqu’à ", "FiberPulse collected ", "FiberPulse a collecté ", " qualified tests", " tests qualifiés", " days.", " jours.", "The median download was ", "Le téléchargement médian était de ", "of the plan speed.", "du débit de l’offre.", "Please create a technical investigation ticket and give me the reference number. I can provide the PDF report with every measurement.", "Merci de créer un ticket d’investigation technique et de me donner sa référence. Je peux fournir le rapport PDF avec chaque mesure.",
			"Evidence collection or subscriber details are not complete yet. Review the draft, but wait until the dossier is ready before sending it as a formal complaint.", "La collecte des preuves ou les coordonnées ne sont pas encore complètes. Relisez le brouillon, mais attendez que le dossier soit prêt avant de l’envoyer comme réclamation formelle.",
		}
	case "de":
		return compactDraftPairs("Anfrage zur technischen Untersuchung", "Guten Tag, Support von ", "Ich bitte um eine technische Untersuchung der Leistung meines Internetanschlusses.", "Kundendaten", "Kontoinhaber:", "Kundennummer:", "Anschlussadresse:", "E-Mail:", "Telefon:", "Gebuchter Dienst", "Anbieter:", "Tarif:", "Beworbener Download: bis zu", "FiberPulse-Messübersicht", "Messzeitraum:", "Qualifizierte Messungen:", "an", "Tag(en)", "Median Download:", "der beworbenen Geschwindigkeit", "Median Upload:", "Mediane Mindestlatenz:", "Ethernet-Tests:", "WLAN-Tests:", "Installation und Testbedingungen", "Modem / ONT des Anbieters:", "Router des Anbieters:", "Zusätzlicher Router:", "Mesh-System:", "Hauptverbindung beim Test:", "Netzaufbau:", "Übliche verbundene Geräte:", "Zusätzliche Hinweise:", "Mit freundlichen Grüßen,", "Nicht angegeben", "Keine", "Die Beweissammlung oder Kundendaten sind noch nicht vollständig. Entwurf prüfen und erst als formelle Beschwerde senden, wenn das Dossier bereit ist.")
	case "es":
		return compactDraftPairs("Solicitud de investigación técnica", "Hola, soporte de ", "Solicito una investigación técnica sobre el rendimiento de mi conexión a Internet.", "Datos del abonado", "Titular:", "Número de cuenta:", "Dirección del servicio:", "Correo:", "Teléfono:", "Servicio contratado", "Proveedor:", "Oferta:", "Descarga anunciada: hasta", "Resumen de seguimiento FiberPulse", "Periodo:", "Mediciones válidas:", "en", "día(s)", "Descarga mediana:", "de la velocidad anunciada", "Subida mediana:", "Latencia mínima mediana:", "Pruebas Ethernet:", "Pruebas Wi-Fi:", "Instalación y condiciones de prueba", "Módem / ONT del proveedor:", "Router del proveedor:", "Router adicional:", "Sistema mesh:", "Conexión principal:", "Topología de red:", "Dispositivos conectados habituales:", "Notas adicionales:", "Atentamente,", "No indicado", "Ninguna", "La recopilación de pruebas o los datos del abonado aún no están completos. Revisa el borrador y espera a que el expediente esté listo antes de enviarlo como reclamación formal.")
	case "pt-BR":
		return compactDraftPairs("Solicitação de investigação técnica", "Olá, suporte da ", "Solicito uma investigação técnica do desempenho da minha conexão com a Internet.", "Dados do assinante", "Titular:", "Número da conta:", "Endereço do serviço:", "E-mail:", "Telefone:", "Serviço contratado", "Provedor:", "Plano:", "Download anunciado: até", "Resumo do monitoramento FiberPulse", "Período:", "Medições qualificadas:", "em", "dia(s)", "Download mediano:", "da velocidade anunciada", "Upload mediano:", "Latência mínima mediana:", "Testes Ethernet:", "Testes Wi-Fi:", "Instalação e condições de teste", "Modem / ONT do provedor:", "Roteador do provedor:", "Roteador adicional:", "Sistema mesh:", "Conexão principal:", "Topologia da rede:", "Dispositivos normalmente conectados:", "Notas adicionais:", "Atenciosamente,", "Não informado", "Nenhuma", "A coleta de evidências ou os dados do assinante ainda não estão completos. Revise o rascunho e aguarde o dossiê ficar pronto antes de enviá-lo como reclamação formal.")
	case "it":
		return compactDraftPairs("Richiesta di verifica tecnica", "Buongiorno, assistenza ", "Richiedo una verifica tecnica delle prestazioni della mia connessione Internet.", "Dati dell’abbonato", "Intestatario:", "Numero cliente:", "Indirizzo del servizio:", "E-mail:", "Telefono:", "Servizio sottoscritto", "Provider:", "Offerta:", "Download pubblicizzato: fino a", "Riepilogo del monitoraggio FiberPulse", "Periodo:", "Misure qualificate:", "in", "giorno/i", "Download mediano:", "della velocità pubblicizzata", "Upload mediano:", "Latenza minima mediana:", "Test Ethernet:", "Test Wi-Fi:", "Installazione e condizioni di test", "Modem / ONT del provider:", "Router del provider:", "Router aggiuntivo:", "Sistema mesh:", "Connessione principale:", "Topologia di rete:", "Dispositivi solitamente connessi:", "Note aggiuntive:", "Cordiali saluti,", "Non indicato", "Nessuna", "La raccolta delle prove o i dati dell’abbonato non sono ancora completi. Controlla la bozza e attendi che il dossier sia pronto prima di inviarlo come reclamo formale.")
	case "hi":
		return compactDraftPairs("तकनीकी जाँच का अनुरोध", "नमस्ते, सहायता टीम ", "मैं अपने इंटरनेट कनेक्शन के प्रदर्शन की तकनीकी जाँच का अनुरोध करता/करती हूँ।", "ग्राहक विवरण", "खाताधारक:", "खाता संख्या:", "सेवा का पता:", "ईमेल:", "फ़ोन:", "सदस्यता सेवा", "प्रदाता:", "प्लान:", "विज्ञापित डाउनलोड: अधिकतम", "FiberPulse निगरानी सारांश", "निगरानी अवधि:", "योग्य माप:", "कुल", "दिन", "मध्य डाउनलोड:", "विज्ञापित गति का", "मध्य अपलोड:", "मध्य न्यूनतम विलंबता:", "Ethernet परीक्षण:", "Wi-Fi परीक्षण:", "इंस्टॉलेशन और परीक्षण की स्थितियाँ", "प्रदाता मॉडेम / ONT:", "प्रदाता राउटर:", "अतिरिक्त राउटर:", "मेश सिस्टम:", "मुख्य परीक्षण कनेक्शन:", "नेटवर्क संरचना:", "आमतौर पर जुड़े डिवाइस:", "अतिरिक्त टिप्पणियाँ:", "सादर,", "नहीं दिया गया", "कोई नहीं", "प्रमाण संग्रह या ग्राहक विवरण अभी पूरे नहीं हैं। मसौदे की समीक्षा करें और औपचारिक शिकायत भेजने से पहले डॉसियर तैयार होने की प्रतीक्षा करें।")
	default:
		return nil
	}
}

func compactDraftPairs(subject, greeting, request, details, holder, account, address, email, phone, service, provider, offer, advertised, summary, period, qualified, across, days, medianDown, percentage, medianUp, latency, ethernet, wifi, conditions, modem, router, additional, mesh, connection, layout, devices, notes, regards, missing, none, warning string) []string {
	return []string{
		"Request for technical investigation", subject, "Dear ", greeting, " Support,", ",", "I am requesting a technical investigation into the performance of my Internet connection.", request,
		"Subscriber details", details, "Account holder:", holder, "Account number:", account, "Service address:", address, "Contact email:", email, "Contact phone:", phone,
		"Subscribed service", service, "Provider:", provider, "Offer:", offer, "Advertised download: up to", advertised, "FiberPulse monitoring summary", summary, "Monitoring period:", period, "Qualified measurements:", qualified, "across", across, "day(s)", days,
		"Median download:", medianDown, "of the advertised speed", percentage, "Median upload:", medianUp, "Median minimum latency:", latency, "Ethernet tests:", ethernet, "Wi-Fi tests:", wifi,
		"Installation and test conditions", conditions, "Provider modem / ONT:", modem, "Provider router:", router, "Additional router:", additional, "Mesh system:", mesh, "Main test connection:", connection, "Network layout:", layout, "Typical connected devices:", devices, "Additional notes:", notes,
		"Kind regards,", regards, "Not provided", missing, "None", none, "Evidence collection or subscriber details are not complete yet. Review the draft, but wait until the dossier is ready before sending it as a formal complaint.", warning,
	}
}
