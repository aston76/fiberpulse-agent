import { useEffect, useState } from "preact/hooks";

export type Locale = "en" | "fr" | "de" | "es" | "pt-BR" | "it" | "hi";

export const localeOptions: Array<{ code: Locale; label: string }> = [
  { code: "en", label: "🇬🇧 English" },
  { code: "fr", label: "🇫🇷 Français" },
  { code: "de", label: "🇩🇪 Deutsch" },
  { code: "es", label: "🇪🇸 Español" },
  { code: "pt-BR", label: "🇧🇷 Português (Brasil)" },
  { code: "it", label: "🇮🇹 Italiano" },
  { code: "hi", label: "🇮🇳 हिन्दी" },
];

type Row = [string, string, string, string, string, string, string];

// English is the canonical source. These translations are bundled with the
// dashboard: changing language never sends UI text or user data to a service.
const rows: Row[] = [
  ["Settings", "Réglages", "Einstellungen", "Ajustes", "Configurações", "Impostazioni", "सेटिंग्स"],
  ["Simple controls", "Commandes simples", "Einfache Steuerung", "Controles simples", "Controles simples", "Controlli semplici", "सरल नियंत्रण"],
  ["Language", "Langue", "Sprache", "Idioma", "Idioma", "Lingua", "भाषा"],
  ["Quit", "Quitter", "Beenden", "Salir", "Sair", "Esci", "बंद करें"],
  ["Online", "En ligne", "Online", "En ligne", "Online", "Online", "ऑनलाइन"],
  ["Offline", "Hors ligne", "Offline", "Sin conexión", "Offline", "Offline", "ऑफ़लाइन"],
  ["Live connection status", "État de la connexion en direct", "Live-Verbindungsstatus", "Estado de conexión en directo", "Estado da conexão ao vivo", "Stato della connessione in tempo reale", "लाइव कनेक्शन स्थिति"],
  ["Your Internet is working", "Votre connexion Internet fonctionne", "Ihre Internetverbindung funktioniert", "Tu conexión a Internet funciona", "Sua Internet está funcionando", "La connessione Internet funziona", "आपका इंटरनेट काम कर रहा है"],
  ["You are offline", "Vous êtes hors ligne", "Sie sind offline", "Estás sin conexión", "Você está offline", "Sei offline", "आप ऑफ़लाइन हैं"],
  ["Your Internet is unstable", "Votre connexion Internet est instable", "Ihre Internetverbindung ist instabil", "Tu conexión es inestable", "Sua Internet está instável", "La connessione è instabile", "आपका इंटरनेट अस्थिर है"],
  ["Internet performance is degraded", "Les performances Internet sont dégradées", "Die Internetleistung ist beeinträchtigt", "El rendimiento de Internet está degradado", "O desempenho da Internet está degradado", "Le prestazioni Internet sono ridotte", "इंटरनेट प्रदर्शन कम है"],
  ["Internet connection detected", "Connexion Internet détectée", "Internetverbindung erkannt", "Conexión a Internet detectada", "Conexão com a Internet detectada", "Connessione Internet rilevata", "इंटरनेट कनेक्शन मिला"],
  ["Checking your Internet…", "Vérification de votre connexion…", "Internetverbindung wird geprüft…", "Comprobando tu conexión…", "Verificando sua Internet…", "Verifica della connessione…", "इंटरनेट की जाँच हो रही है…"],
  ["Run your first test to measure the real performance.", "Lancez votre premier test pour mesurer les performances réelles.", "Starten Sie den ersten Test, um die tatsächliche Leistung zu messen.", "Realiza la primera prueba para medir el rendimiento real.", "Faça o primeiro teste para medir o desempenho real.", "Avvia il primo test per misurare le prestazioni reali.", "वास्तविक प्रदर्शन मापने के लिए पहला परीक्षण चलाएँ।"],
  ["DOWNLOAD", "TÉLÉCHARGEMENT", "DOWNLOAD", "DESCARGA", "DOWNLOAD", "DOWNLOAD", "डाउनलोड"],
  ["UPLOAD", "ENVOI", "UPLOAD", "SUBIDA", "UPLOAD", "UPLOAD", "अपलोड"],
  ["LATENCY", "LATENCE", "LATENZ", "LATENCIA", "LATÊNCIA", "LATENZA", "विलंबता"],
  ["Run speed test", "Lancer le test de débit", "Geschwindigkeitstest starten", "Iniciar prueba de velocidad", "Iniciar teste de velocidade", "Avvia test di velocità", "स्पीड टेस्ट चलाएँ"],
  ["Measures download, upload and latency. Results stay on this device.", "Mesure le téléchargement, l’envoi et la latence. Les résultats restent sur cet appareil.", "Misst Download, Upload und Latenz. Die Ergebnisse bleiben auf diesem Gerät.", "Mide descarga, subida y latencia. Los resultados permanecen en este dispositivo.", "Mede download, upload e latência. Os resultados ficam neste dispositivo.", "Misura download, upload e latenza. I risultati restano su questo dispositivo.", "डाउनलोड, अपलोड और विलंबता मापता है। परिणाम इसी डिवाइस पर रहते हैं।"],
  ["AUTOMATIC MONITORING", "SURVEILLANCE AUTOMATIQUE", "AUTOMATISCHE ÜBERWACHUNG", "SUPERVISIÓN AUTOMÁTICA", "MONITORAMENTO AUTOMÁTICO", "MONITORAGGIO AUTOMATICO", "स्वचालित निगरानी"],
  ["Automatic monitoring", "Surveillance automatique", "Automatische Überwachung", "Supervisión automática", "Monitoramento automático", "Monitoraggio automatico", "स्वचालित निगरानी"],
  ["Active", "Actif", "Aktiv", "Activo", "Ativo", "Attivo", "सक्रिय"],
  ["Paused", "En pause", "Pausiert", "En pausa", "Pausado", "In pausa", "रुका हुआ"],
  ["Pause", "Mettre en pause", "Pausieren", "Pausar", "Pausar", "Pausa", "रोकें"],
  ["Resume", "Reprendre", "Fortsetzen", "Reanudar", "Retomar", "Riprendi", "जारी रखें"],
  ["ACTIVE ISSUES", "PROBLÈMES ACTIFS", "AKTIVE PROBLEME", "PROBLEMAS ACTIVOS", "PROBLEMAS ATIVOS", "PROBLEMI ATTIVI", "सक्रिय समस्याएँ"],
  ["No problem detected", "Aucun problème détecté", "Kein Problem erkannt", "No se detectaron problemas", "Nenhum problema detectado", "Nessun problema rilevato", "कोई समस्या नहीं मिली"],
  ["YOUR PLAN", "VOTRE OFFRE", "IHR TARIF", "TU TARIFA", "SEU PLANO", "IL TUO PIANO", "आपका प्लान"],
  ["Not selected", "Non sélectionnée", "Nicht ausgewählt", "No seleccionada", "Não selecionado", "Non selezionato", "चुना नहीं गया"],
  ["Choose", "Choisir", "Auswählen", "Elegir", "Escolher", "Scegli", "चुनें"],
  ["Change", "Modifier", "Ändern", "Cambiar", "Alterar", "Cambia", "बदलें"],
  ["Your speed over time", "Votre débit dans le temps", "Ihre Geschwindigkeit im Zeitverlauf", "Tu velocidad a lo largo del tiempo", "Sua velocidade ao longo do tempo", "La velocità nel tempo", "समय के साथ आपकी गति"],
  ["Simple performance history", "Historique simple des performances", "Einfacher Leistungsverlauf", "Historial de rendimiento sencillo", "Histórico simples de desempenho", "Cronologia semplice delle prestazioni", "सरल प्रदर्शन इतिहास"],
  ["Your history will appear here", "Votre historique apparaîtra ici", "Ihr Verlauf erscheint hier", "Tu historial aparecerá aquí", "Seu histórico aparecerá aqui", "La cronologia apparirà qui", "आपका इतिहास यहाँ दिखेगा"],
  ["Run two tests to see the trend.", "Lancez deux tests pour voir la tendance.", "Führen Sie zwei Tests aus, um den Trend zu sehen.", "Realiza dos pruebas para ver la tendencia.", "Faça dois testes para ver a tendência.", "Esegui due test per vedere l’andamento.", "रुझान देखने के लिए दो परीक्षण चलाएँ।"],
  ["DIAGNOSTICS & EVIDENCE", "DIAGNOSTIC ET PREUVES", "DIAGNOSE & NACHWEISE", "DIAGNÓSTICO Y PRUEBAS", "DIAGNÓSTICO E EVIDÊNCIAS", "DIAGNOSTICA E PROVE", "निदान और प्रमाण"],
  ["Details and reports", "Détails et rapports", "Details und Berichte", "Detalles e informes", "Detalhes e relatórios", "Dettagli e rapporti", "विवरण और रिपोर्ट"],
  ["Connection intelligence", "Analyse de la connexion", "Verbindungsanalyse", "Análisis de conexión", "Análise da conexão", "Analisi della connessione", "कनेक्शन विश्लेषण"],
  ["Your diagnostic workspace", "Votre espace de diagnostic", "Ihr Diagnosebereich", "Tu espacio de diagnóstico", "Seu espaço de diagnóstico", "Il tuo spazio diagnostico", "आपका निदान कार्यक्षेत्र"],
  ["Network context", "Contexte réseau", "Netzwerkkontext", "Contexto de red", "Contexto da rede", "Contesto di rete", "नेटवर्क संदर्भ"],
  ["Latest result", "Dernier résultat", "Letztes Ergebnis", "Último resultado", "Resultado mais recente", "Ultimo risultato", "नवीनतम परिणाम"],
  ["Your baseline", "Votre référence", "Ihre Referenz", "Tu referencia", "Sua referência", "Il tuo riferimento", "आपका आधार"],
  ["Plan diagnosis", "Diagnostic de l’offre", "Tarifdiagnose", "Diagnóstico de tarifa", "Diagnóstico do plano", "Diagnosi del piano", "प्लान निदान"],
  ["Country", "Pays", "Land", "País", "País", "Paese", "देश"],
  ["Provider", "Fournisseur", "Anbieter", "Proveedor", "Provedor", "Operatore", "प्रदाता"],
  ["Offer", "Offre", "Tarif", "Oferta", "Oferta", "Offerta", "ऑफ़र"],
  ["Your Internet plan", "Votre offre Internet", "Ihr Internettarif", "Tu tarifa de Internet", "Seu plano de Internet", "Il tuo piano Internet", "आपका इंटरनेट प्लान"],
  ["What do you pay for?", "Quelle offre payez-vous ?", "Welchen Tarif bezahlen Sie?", "¿Qué tarifa pagas?", "Qual plano você paga?", "Quale piano paghi?", "आप किस प्लान के लिए भुगतान करते हैं?"],
  ["My provider / plan is not listed", "Mon fournisseur / offre n’est pas répertorié", "Mein Anbieter/Tarif ist nicht aufgeführt", "Mi proveedor/tarifa no aparece", "Meu provedor/plano não está listado", "Il mio operatore/piano non è elencato", "मेरा प्रदाता/प्लान सूची में नहीं है"],
  ["Provider name", "Nom du fournisseur", "Anbietername", "Nombre del proveedor", "Nome do provedor", "Nome dell’operatore", "प्रदाता का नाम"],
  ["Offer name", "Nom de l’offre", "Tarifname", "Nombre de la oferta", "Nome da oferta", "Nome dell’offerta", "ऑफ़र का नाम"],
  ["Advertised download (Mbps)", "Débit descendant annoncé (Mbit/s)", "Beworbener Download (Mbit/s)", "Descarga anunciada (Mbps)", "Download anunciado (Mbps)", "Download pubblicizzato (Mbps)", "विज्ञापित डाउनलोड (Mbps)"],
  ["Advertised upload (optional)", "Débit montant annoncé (facultatif)", "Beworbener Upload (optional)", "Subida anunciada (opcional)", "Upload anunciado (opcional)", "Upload pubblicizzato (facoltativo)", "विज्ञापित अपलोड (वैकल्पिक)"],
  ["Save plan", "Enregistrer l’offre", "Tarif speichern", "Guardar tarifa", "Salvar plano", "Salva piano", "प्लान सहेजें"],
  ["Remove", "Supprimer", "Entfernen", "Eliminar", "Remover", "Rimuovi", "हटाएँ"],
  ["Cancel", "Annuler", "Abbrechen", "Cancelar", "Cancelar", "Annulla", "रद्द करें"],
  ["Open provider source ↗", "Ouvrir la source du fournisseur ↗", "Anbieterquelle öffnen ↗", "Abrir fuente del proveedor ↗", "Abrir fonte do provedor ↗", "Apri fonte dell’operatore ↗", "प्रदाता स्रोत खोलें ↗"],
  ["Official catalog checked", "Catalogue officiel vérifié", "Offizieller Katalog geprüft", "Catálogo oficial verificado", "Catálogo oficial verificado", "Catalogo ufficiale verificato", "आधिकारिक कैटलॉग सत्यापित"],
  ["Internet speed tests", "Tests de débit Internet", "Internet-Geschwindigkeitstests", "Pruebas de velocidad", "Testes de velocidade", "Test di velocità Internet", "इंटरनेट स्पीड टेस्ट"],
  ["Allowed permanently", "Autorisé durablement", "Dauerhaft erlaubt", "Permitido permanentemente", "Permitido permanentemente", "Consentito permanentemente", "स्थायी रूप से अनुमत"],
  ["Disabled", "Désactivé", "Deaktiviert", "Desactivado", "Desativado", "Disattivato", "बंद"],
  ["Enable", "Activer", "Aktivieren", "Activar", "Ativar", "Attiva", "सक्रिय करें"],
  ["Disable", "Désactiver", "Deaktivieren", "Desactivar", "Desativar", "Disattiva", "अक्षम करें"],
  ["Anonymous sharing", "Partage anonyme", "Anonyme Freigabe", "Uso compartido anónimo", "Compartilhamento anônimo", "Condivisione anonima", "अनाम साझाकरण"],
  ["Enabled", "Activé", "Aktiviert", "Activado", "Ativado", "Attivato", "सक्रिय"],
  ["Optional and disabled", "Facultatif et désactivé", "Optional und deaktiviert", "Opcional y desactivado", "Opcional e desativado", "Facoltativo e disattivato", "वैकल्पिक और बंद"],
  ["Subscriber profile", "Profil de l’abonné", "Teilnehmerprofil", "Perfil del abonado", "Perfil do assinante", "Profilo dell’abbonato", "ग्राहक प्रोफ़ाइल"],
  ["Edit", "Modifier", "Bearbeiten", "Editar", "Editar", "Modifica", "संपादित करें"],
  ["Complete", "Compléter", "Vervollständigen", "Completar", "Completar", "Completa", "पूरा करें"],
  ["One-time setup", "Configuration unique", "Einmalige Einrichtung", "Configuración inicial", "Configuração inicial", "Configurazione iniziale", "एक बार की सेटअप"],
  ["Allow Internet speed tests?", "Autoriser les tests de débit Internet ?", "Internet-Geschwindigkeitstests erlauben?", "¿Permitir pruebas de velocidad?", "Permitir testes de velocidade?", "Consentire i test di velocità?", "इंटरनेट स्पीड टेस्ट की अनुमति दें?"],
  ["Not now", "Pas maintenant", "Nicht jetzt", "Ahora no", "Agora não", "Non ora", "अभी नहीं"],
  ["Allow permanently", "Autoriser durablement", "Dauerhaft erlauben", "Permitir permanentemente", "Permitir permanentemente", "Consenti permanentemente", "स्थायी अनुमति दें"],
  ["Read M-Lab privacy policy", "Lire la politique de confidentialité M-Lab", "M-Lab-Datenschutzrichtlinie lesen", "Leer la política de privacidad de M-Lab", "Ler a política de privacidade do M-Lab", "Leggi l’informativa privacy M-Lab", "M-Lab गोपनीयता नीति पढ़ें"],
  ["Optional", "Facultatif", "Optional", "Opcional", "Opcional", "Facoltativo", "वैकल्पिक"],
  ["Help improve Internet data?", "Aider à améliorer les données Internet ?", "Internetdaten verbessern helfen?", "¿Ayudar a mejorar los datos de Internet?", "Ajudar a melhorar os dados da Internet?", "Aiutare a migliorare i dati Internet?", "इंटरनेट डेटा सुधारने में मदद करें?"],
  ["Enable sharing", "Activer le partage", "Freigabe aktivieren", "Activar uso compartido", "Ativar compartilhamento", "Attiva condivisione", "साझाकरण सक्षम करें"],
  ["Before the speed test", "Avant le test de débit", "Vor dem Geschwindigkeitstest", "Antes de la prueba", "Antes do teste", "Prima del test", "स्पीड टेस्ट से पहले"],
  ["Disconnect your VPN first", "Déconnectez d’abord votre VPN", "Trennen Sie zuerst das VPN", "Desconecta primero la VPN", "Desconecte primeiro a VPN", "Disconnetti prima la VPN", "पहले VPN बंद करें"],
  ["Improve your Wi-Fi measurement", "Améliorez votre mesure Wi-Fi", "WLAN-Messung verbessern", "Mejora la medición Wi-Fi", "Melhore a medição Wi-Fi", "Migliora la misurazione Wi-Fi", "Wi-Fi माप बेहतर करें"],
  ["Best method: Ethernet cable", "Meilleure méthode : câble Ethernet", "Beste Methode: Ethernet-Kabel", "Mejor método: cable Ethernet", "Melhor método: cabo Ethernet", "Metodo migliore: cavo Ethernet", "सर्वोत्तम तरीका: ईथरनेट केबल"],
  ["Continue on Wi-Fi", "Continuer en Wi-Fi", "Mit WLAN fortfahren", "Continuar con Wi-Fi", "Continuar no Wi-Fi", "Continua con Wi-Fi", "Wi-Fi पर जारी रखें"],
  ["Provider-ready dossier", "Dossier prêt pour le fournisseur", "Anbieterfertige Unterlagen", "Expediente para el proveedor", "Dossiê para o provedor", "Dossier per l’operatore", "प्रदाता के लिए तैयार फ़ाइल"],
  ["Subscriber and installation profile", "Profil de l’abonné et de l’installation", "Teilnehmer- und Installationsprofil", "Perfil del abonado y la instalación", "Perfil do assinante e da instalação", "Profilo abbonato e installazione", "ग्राहक और इंस्टॉलेशन प्रोफ़ाइल"],
  ["Account holder *", "Titulaire du compte *", "Kontoinhaber *", "Titular de la cuenta *", "Titular da conta *", "Intestatario account *", "खाता धारक *"],
  ["Provider account number *", "Numéro de compte fournisseur *", "Anbieter-Kundennummer *", "Número de cuenta del proveedor *", "Número da conta do provedor *", "Numero cliente operatore *", "प्रदाता खाता संख्या *"],
  ["Service address *", "Adresse du service *", "Anschlussadresse *", "Dirección del servicio *", "Endereço do serviço *", "Indirizzo del servizio *", "सेवा पता *"],
  ["Save local profile", "Enregistrer le profil local", "Lokales Profil speichern", "Guardar perfil local", "Salvar perfil local", "Salva profilo locale", "स्थानीय प्रोफ़ाइल सहेजें"],
  ["Provider complaint draft", "Brouillon de réclamation au fournisseur", "Beschwerdeentwurf an den Anbieter", "Borrador de reclamación", "Rascunho de reclamação", "Bozza di reclamo", "प्रदाता शिकायत मसौदा"],
  ["Recipient", "Destinataire", "Empfänger", "Destinatario", "Destinatário", "Destinatario", "प्राप्तकर्ता"],
  ["Subject", "Objet", "Betreff", "Asunto", "Assunto", "Oggetto", "विषय"],
  ["Email", "E-mail", "E-Mail", "Correo", "E-mail", "E-mail", "ईमेल"],
  ["Copy email", "Copier l’e-mail", "E-Mail kopieren", "Copiar correo", "Copiar e-mail", "Copia e-mail", "ईमेल कॉपी करें"],
  ["Copy call script", "Copier le script d’appel", "Anrufskript kopieren", "Copiar guion de llamada", "Copiar roteiro de chamada", "Copia script chiamata", "कॉल स्क्रिप्ट कॉपी करें"],
  ["Evidence package", "Dossier de preuves", "Nachweispaket", "Paquete de pruebas", "Pacote de evidências", "Pacchetto di prove", "प्रमाण पैकेज"],
  ["Professional report", "Rapport professionnel", "Professioneller Bericht", "Informe profesional", "Relatório profissional", "Rapporto professionale", "पेशेवर रिपोर्ट"],
  ["Raw measurement data", "Données de mesure brutes", "Rohmessdaten", "Datos de medición sin procesar", "Dados brutos de medição", "Dati grezzi di misurazione", "कच्चा मापन डेटा"],
  ["Open email app", "Ouvrir l’application e-mail", "E-Mail-App öffnen", "Abrir aplicación de correo", "Abrir aplicativo de e-mail", "Apri app e-mail", "ईमेल ऐप खोलें"],
  ["Private by default · Data stored locally", "Privé par défaut · Données stockées localement", "Standardmäßig privat · Daten lokal gespeichert", "Privado por defecto · Datos locales", "Privado por padrão · Dados locais", "Privato per impostazione predefinita · Dati locali", "डिफ़ॉल्ट रूप से निजी · डेटा स्थानीय रूप से संग्रहीत"],
  ["Last speed test", "Dernier test de débit", "Letzter Geschwindigkeitstest", "Última prueba de velocidad", "Último teste de velocidade", "Ultimo test di velocità", "अंतिम स्पीड टेस्ट"],
  ["Next check", "Prochaine vérification", "Nächste Prüfung", "Próxima comprobación", "Próxima verificação", "Prossimo controllo", "अगली जाँच"],
  ["test", "test", "Test", "prueba", "teste", "test", "परीक्षण"],
  ["tests", "tests", "Tests", "pruebas", "testes", "test", "परीक्षण"],
  ["issue", "problème", "Problem", "problema", "problema", "problema", "समस्या"],
  ["issues", "problèmes", "Probleme", "problemas", "problemas", "problemi", "समस्याएँ"],
  ["Up to", "Jusqu’à", "Bis zu", "Hasta", "Até", "Fino a", "अधिकतम"],
  ["Average", "Moyenne", "Durchschnittlich", "Promedio", "Média", "Media", "औसत"],
  ["Typically", "Généralement", "Typischerweise", "Normalmente", "Normalmente", "Tipicamente", "सामान्यतः"],
  ["Philippines", "Philippines", "Philippinen", "Filipinas", "Filipinas", "Filippine", "फ़िलीपींस"],
  ["United States", "États-Unis", "Vereinigte Staaten", "Estados Unidos", "Estados Unidos", "Stati Uniti", "संयुक्त राज्य अमेरिका"],
  ["United Kingdom", "Royaume-Uni", "Vereinigtes Königreich", "Reino Unido", "Reino Unido", "Regno Unito", "यूनाइटेड किंगडम"],
  ["Germany", "Allemagne", "Deutschland", "Alemania", "Alemanha", "Germania", "जर्मनी"],
  ["France", "France", "Frankreich", "Francia", "França", "Francia", "फ़्रांस"],
  ["Australia", "Australie", "Australien", "Australia", "Austrália", "Australia", "ऑस्ट्रेलिया"],
  ["Canada", "Canada", "Kanada", "Canadá", "Canadá", "Canada", "कनाडा"],
  ["Switzerland", "Suisse", "Schweiz", "Suiza", "Suíça", "Svizzera", "स्विट्ज़रलैंड"],
  ["Spain", "Espagne", "Spanien", "España", "Espanha", "Spagna", "स्पेन"],
  ["Brazil", "Brésil", "Brasilien", "Brasil", "Brasil", "Brasile", "ब्राज़ील"],
  ["India", "Inde", "Indien", "India", "Índia", "India", "भारत"],
  ["Choose once. FiberPulse will remember your decision permanently on this Mac.", "Choisissez une fois. FiberPulse conservera ce choix sur cet appareil.", "Einmal auswählen. FiberPulse speichert die Entscheidung dauerhaft auf diesem Gerät.", "Elige una vez. FiberPulse recordará la decisión en este dispositivo.", "Escolha uma vez. O FiberPulse lembrará a decisão neste dispositivo.", "Scegli una volta. FiberPulse ricorderà la decisione su questo dispositivo.", "एक बार चुनें। FiberPulse इस डिवाइस पर निर्णय याद रखेगा।"],
  ["3 automatic tests per day", "3 tests automatiques par jour", "3 automatische Tests pro Tag", "3 pruebas automáticas al día", "3 testes automáticos por dia", "3 test automatici al giorno", "प्रतिदिन 3 स्वचालित परीक्षण"],
  ["Never more than 4 automatic tests in 24 hours.", "Jamais plus de 4 tests automatiques en 24 heures.", "Nie mehr als 4 automatische Tests in 24 Stunden.", "Nunca más de 4 pruebas automáticas en 24 horas.", "Nunca mais de 4 testes automáticos em 24 horas.", "Mai più di 4 test automatici in 24 ore.", "24 घंटे में 4 से अधिक स्वचालित परीक्षण कभी नहीं।"],
  ["Tests use download and upload data", "Les tests utilisent des données descendantes et montantes", "Tests nutzen Download- und Upload-Daten", "Las pruebas usan datos de descarga y subida", "Os testes usam dados de download e upload", "I test usano dati di download e upload", "परीक्षण डाउनलोड और अपलोड डेटा का उपयोग करते हैं"],
  ["Automatic tests stop on metered or roaming networks.", "Les tests automatiques s’arrêtent sur les réseaux limités ou en itinérance.", "Automatische Tests stoppen bei getakteten Netzen oder Roaming.", "Las pruebas automáticas se detienen en redes de pago por uso o itinerancia.", "Os testes automáticos param em redes limitadas ou em roaming.", "I test automatici si fermano su reti a consumo o in roaming.", "मीटर्ड या रोमिंग नेटवर्क पर स्वचालित परीक्षण रुक जाते हैं।"],
  ["M-Lab receives and publishes test data", "M-Lab reçoit et publie les données de test", "M-Lab empfängt und veröffentlicht Testdaten", "M-Lab recibe y publica los datos de prueba", "O M-Lab recebe e publica os dados do teste", "M-Lab riceve e pubblica i dati del test", "M-Lab परीक्षण डेटा प्राप्त और प्रकाशित करता है"],
  ["This includes your public IP. FiberPulse cannot erase M-Lab history.", "Cela inclut votre IP publique. FiberPulse ne peut pas effacer l’historique M-Lab.", "Dies umfasst Ihre öffentliche IP. FiberPulse kann den M-Lab-Verlauf nicht löschen.", "Esto incluye tu IP pública. FiberPulse no puede borrar el historial de M-Lab.", "Isso inclui seu IP público. O FiberPulse não pode apagar o histórico do M-Lab.", "Include il tuo IP pubblico. FiberPulse non può cancellare la cronologia M-Lab.", "इसमें आपका सार्वजनिक IP शामिल है। FiberPulse M-Lab इतिहास नहीं मिटा सकता।"],
  ["I understand how the tests work and M-Lab’s data policy.", "Je comprends le fonctionnement des tests et la politique de données de M-Lab.", "Ich verstehe die Tests und die Datenrichtlinie von M-Lab.", "Entiendo cómo funcionan las pruebas y la política de datos de M-Lab.", "Entendo como os testes funcionam e a política de dados do M-Lab.", "Comprendo il funzionamento dei test e la politica dati di M-Lab.", "मैं परीक्षणों और M-Lab की डेटा नीति को समझता/समझती हूँ।"],
  ["Share privacy-preserving measurement results with the public FiberPulse Observatory. This is separate from speed testing.", "Partagez des mesures respectueuses de la vie privée avec l’Observatoire public FiberPulse. Ce choix est distinct des tests de débit.", "Teilen Sie datenschutzfreundliche Messwerte mit dem öffentlichen FiberPulse-Observatorium. Dies ist von Geschwindigkeitstests getrennt.", "Comparte mediciones que protegen la privacidad con el Observatorio público FiberPulse. Es independiente de las pruebas de velocidad.", "Compartilhe medições com proteção de privacidade no Observatório público FiberPulse. Isso é separado dos testes de velocidade.", "Condividi misure rispettose della privacy con l’Osservatorio pubblico FiberPulse. È separato dai test di velocità.", "गोपनीयता-सुरक्षित मापन सार्वजनिक FiberPulse Observatory से साझा करें। यह स्पीड टेस्ट से अलग है।"],
  ["What becomes public", "Ce qui devient public", "Was öffentlich wird", "Qué se hace público", "O que se torna público", "Cosa diventa pubblico", "क्या सार्वजनिक होता है"],
  ["No account or device profile", "Aucun compte ni profil d’appareil", "Kein Konto- oder Geräteprofil", "Sin cuenta ni perfil del dispositivo", "Sem conta ou perfil do dispositivo", "Nessun account o profilo dispositivo", "कोई खाता या डिवाइस प्रोफ़ाइल नहीं"],
  ["You stay in control", "Vous gardez le contrôle", "Sie behalten die Kontrolle", "Tú mantienes el control", "Você mantém o controle", "Mantieni il controllo", "नियंत्रण आपके पास रहता है"],
  ["I understand which minimal fields are shared.", "Je comprends quels champs minimaux sont partagés.", "Ich verstehe, welche minimalen Felder geteilt werden.", "Entiendo qué campos mínimos se comparten.", "Entendo quais campos mínimos são compartilhados.", "Comprendo quali campi minimi vengono condivisi.", "मैं समझता/समझती हूँ कि कौन से न्यूनतम फ़ील्ड साझा होते हैं।"],
  ["Turn off the VPN application", "Désactivez l’application VPN", "VPN-Anwendung ausschalten", "Desactiva la aplicación VPN", "Desative o aplicativo VPN", "Disattiva l’app VPN", "VPN ऐप बंद करें"],
  ["Then start the test again", "Relancez ensuite le test", "Danach den Test erneut starten", "Después inicia de nuevo la prueba", "Depois inicie o teste novamente", "Poi avvia di nuovo il test", "फिर परीक्षण दोबारा शुरू करें"],
  ["If you stay on Wi-Fi", "Si vous restez en Wi-Fi", "Wenn Sie WLAN verwenden", "Si sigues con Wi-Fi", "Se continuar no Wi-Fi", "Se resti in Wi-Fi", "यदि आप Wi-Fi पर रहें"],
  ["Pause other traffic", "Mettez les autres activités en pause", "Anderen Datenverkehr pausieren", "Pausa el resto del tráfico", "Pause o restante do tráfego", "Metti in pausa l’altro traffico", "अन्य ट्रैफ़िक रोकें"],
  ["FiberPulse compares your measurements with the advertised speed of your offer. Consumer plans advertise \"up to\" speeds, so the verdict stays conservative.", "FiberPulse compare vos mesures au débit annoncé de votre offre. Les offres grand public indiquent un débit maximal : l’évaluation reste donc prudente.", "FiberPulse vergleicht Ihre Messungen mit der beworbenen Tarifgeschwindigkeit. Bei „bis zu“-Angaben bleibt die Bewertung vorsichtig.", "FiberPulse compara tus mediciones con la velocidad anunciada. Como se anuncia «hasta», la evaluación es prudente.", "O FiberPulse compara suas medições com a velocidade anunciada. Como os planos anunciam “até”, a avaliação é conservadora.", "FiberPulse confronta le misure con la velocità pubblicizzata. Poiché i piani indicano “fino a”, la valutazione resta prudente.", "FiberPulse आपके मापन की तुलना विज्ञापित गति से करता है। “अधिकतम” गति के कारण निष्कर्ष सावधान रहता है।"],
  ["Use the speed written on your latest bill or contract. FiberPulse marks this as subscriber-entered in the report.", "Utilisez le débit indiqué sur votre dernière facture ou votre contrat. Le rapport précisera que l’offre a été saisie par l’abonné.", "Verwenden Sie die Geschwindigkeit aus der letzten Rechnung oder dem Vertrag. Der Bericht kennzeichnet sie als Nutzereingabe.", "Usa la velocidad de tu última factura o contrato. El informe indicará que la introdujo el abonado.", "Use a velocidade da fatura ou contrato mais recente. O relatório indicará que foi informada pelo assinante.", "Usa la velocità dell’ultima fattura o contratto. Il rapporto la indica come inserita dall’abbonato.", "नवीनतम बिल या अनुबंध की गति दर्ज करें। रिपोर्ट इसे ग्राहक द्वारा दर्ज बताएगी।"],
  ["Active link", "Liaison active", "Aktive Verbindung", "Enlace activo", "Link ativo", "Collegamento attivo", "सक्रिय लिंक"],
  ["VPN / proxy", "VPN / proxy", "VPN / Proxy", "VPN / proxy", "VPN / proxy", "VPN / proxy", "VPN / प्रॉक्सी"],
  ["Metered network", "Réseau limité", "Getaktetes Netzwerk", "Red de uso medido", "Rede limitada", "Rete a consumo", "मीटर्ड नेटवर्क"],
  ["Confidence", "Confiance", "Vertrauen", "Confianza", "Confiança", "Affidabilità", "विश्वसनीयता"],
  ["Evidence eligible", "Admissible comme preuve", "Als Nachweis geeignet", "Apto como prueba", "Elegível como evidência", "Idoneo come prova", "प्रमाण के योग्य"],
  ["Test provider", "Fournisseur du test", "Testanbieter", "Proveedor de prueba", "Provedor do teste", "Fornitore del test", "परीक्षण प्रदाता"],
  ["Qualified tests", "Tests qualifiés", "Qualifizierte Tests", "Pruebas válidas", "Testes qualificados", "Test qualificati", "योग्य परीक्षण"],
  ["Median download", "Débit descendant médian", "Medianer Download", "Descarga mediana", "Download mediano", "Download mediano", "मध्य डाउनलोड"],
  ["Maturity", "Maturité", "Reifegrad", "Madurez", "Maturidade", "Maturità", "परिपक्वता"],
  ["Network incidents", "Incidents réseau", "Netzwerkvorfälle", "Incidencias de red", "Incidentes de rede", "Incidenti di rete", "नेटवर्क घटनाएँ"],
  ["Latest comparison", "Dernière comparaison", "Letzter Vergleich", "Última comparación", "Comparação mais recente", "Ultimo confronto", "नवीनतम तुलना"],
  ["Select your provider and offer to compare measured performance with what you pay for.", "Sélectionnez votre fournisseur et votre offre pour comparer les performances mesurées à ce que vous payez.", "Wählen Sie Anbieter und Tarif, um die Messleistung mit Ihrem Vertrag zu vergleichen.", "Selecciona proveedor y tarifa para comparar el rendimiento medido con lo que pagas.", "Selecione provedor e plano para comparar o desempenho medido com o contratado.", "Seleziona operatore e piano per confrontare le prestazioni misurate con ciò che paghi.", "मापे गए प्रदर्शन की तुलना अपने भुगतान वाले प्लान से करने के लिए प्रदाता और ऑफ़र चुनें।"],
  ["Take your results with you", "Emportez vos résultats", "Ergebnisse mitnehmen", "Lleva tus resultados contigo", "Leve seus resultados", "Porta con te i risultati", "अपने परिणाम साथ लें"],
  ["Create a polished report for your provider or download the underlying measurements for deeper analysis.", "Créez un rapport soigné pour votre fournisseur ou téléchargez les mesures pour une analyse approfondie.", "Erstellen Sie einen professionellen Anbieterbericht oder laden Sie die Messdaten zur Analyse herunter.", "Crea un informe profesional o descarga las mediciones para analizarlas.", "Crie um relatório profissional ou baixe as medições para análise.", "Crea un rapporto professionale o scarica le misure per analizzarle.", "प्रदाता के लिए रिपोर्ट बनाएँ या गहन विश्लेषण हेतु मापन डाउनलोड करें।"],
  ["Seven-day provider case", "Dossier fournisseur sur sept jours", "Siebentägiger Anbieterfall", "Expediente de siete días", "Dossiê de sete dias", "Pratica di sette giorni", "सात दिन का प्रदाता मामला"],
  ["FiberPulse consolidates 3 qualified tests per day, your installation details and your subscribed offer into a professional provider dossier.", "FiberPulse regroupe 3 tests qualifiés par jour, les détails de votre installation et votre offre dans un dossier professionnel.", "FiberPulse bündelt täglich 3 qualifizierte Tests, Installationsdaten und Tarif in professionellen Unterlagen.", "FiberPulse reúne 3 pruebas válidas al día, los datos de instalación y la tarifa en un expediente profesional.", "O FiberPulse reúne 3 testes qualificados por dia, dados da instalação e plano em um dossiê profissional.", "FiberPulse riunisce 3 test qualificati al giorno, dati dell’installazione e piano in un dossier professionale.", "FiberPulse प्रतिदिन 3 योग्य परीक्षण, इंस्टॉलेशन विवरण और प्लान को पेशेवर फ़ाइल में जोड़ता है।"],
  ["Observed days", "Jours observés", "Beobachtete Tage", "Días observados", "Dias observados", "Giorni osservati", "देखे गए दिन"],
  ["Preview & copy", "Prévisualiser et copier", "Vorschau & kopieren", "Vista previa y copiar", "Visualizar e copiar", "Anteprima e copia", "पूर्वावलोकन और कॉपी"],
  ["These details are saved only in your local FiberPulse database. They are inserted into your complaint files, never into technical logs or anonymous sharing.", "Ces informations sont enregistrées uniquement dans la base locale FiberPulse. Elles figurent dans vos réclamations, jamais dans les journaux techniques ni le partage anonyme.", "Diese Angaben werden nur lokal gespeichert und nur in Beschwerden verwendet, nie in technischen Protokollen oder anonymen Daten.", "Estos datos se guardan solo localmente y se usan en reclamaciones, nunca en registros técnicos ni datos anónimos.", "Esses dados ficam apenas no banco local e são usados em reclamações, nunca em logs ou compartilhamento anônimo.", "Questi dati restano nel database locale e sono usati nei reclami, mai nei log o nella condivisione anonima.", "ये विवरण केवल स्थानीय डेटाबेस में रहते हैं और शिकायत फ़ाइलों में उपयोग होते हैं, तकनीकी लॉग या अनाम साझाकरण में नहीं।"],
  ["Your email", "Votre e-mail", "Ihre E-Mail", "Tu correo", "Seu e-mail", "La tua e-mail", "आपका ईमेल"],
  ["Your phone", "Votre téléphone", "Ihre Telefonnummer", "Tu teléfono", "Seu telefone", "Il tuo telefono", "आपका फ़ोन"],
  ["Provider modem / ONT", "Modem / ONT du fournisseur", "Anbieter-Modem / ONT", "Módem / ONT del proveedor", "Modem / ONT do provedor", "Modem / ONT operatore", "प्रदाता मॉडेम / ONT"],
  ["Provider router", "Routeur du fournisseur", "Anbieter-Router", "Router del proveedor", "Roteador do provedor", "Router operatore", "प्रदाता राउटर"],
  ["Main test connection", "Connexion principale du test", "Hauptverbindung für Tests", "Conexión principal de prueba", "Conexão principal do teste", "Connessione principale del test", "मुख्य परीक्षण कनेक्शन"],
  ["Network layout", "Topologie du réseau", "Netzwerkaufbau", "Configuración de red", "Topologia da rede", "Configurazione di rete", "नेटवर्क संरचना"],
  ["I use an additional router", "J’utilise un routeur supplémentaire", "Ich nutze einen zusätzlichen Router", "Uso un router adicional", "Uso um roteador adicional", "Uso un router aggiuntivo", "मैं अतिरिक्त राउटर उपयोग करता/करती हूँ"],
  ["I use a mesh system", "J’utilise un système mesh", "Ich nutze ein Mesh-System", "Uso un sistema mesh", "Uso um sistema mesh", "Uso un sistema mesh", "मैं मेश सिस्टम उपयोग करता/करती हूँ"],
  ["Typical connected devices", "Appareils habituellement connectés", "Typische Zahl verbundener Geräte", "Dispositivos conectados habituales", "Dispositivos conectados normalmente", "Dispositivi normalmente connessi", "आमतौर पर जुड़े डिवाइस"],
  ["Useful technical notes", "Notes techniques utiles", "Nützliche technische Hinweise", "Notas técnicas útiles", "Notas técnicas úteis", "Note tecniche utili", "उपयोगी तकनीकी नोट्स"],
  ["Your choices are saved on this device. FiberPulse will not ask again automatically.", "Vos choix sont enregistrés sur cet appareil. FiberPulse ne vous les redemandera pas automatiquement.", "Ihre Auswahl wird auf diesem Gerät gespeichert. FiberPulse fragt nicht automatisch erneut.", "Tus elecciones se guardan en este dispositivo. FiberPulse no volverá a preguntar automáticamente.", "Suas escolhas ficam neste dispositivo. O FiberPulse não perguntará novamente automaticamente.", "Le scelte vengono salvate su questo dispositivo. FiberPulse non chiederà di nuovo automaticamente.", "आपकी पसंद इस डिवाइस पर सहेजी जाती है। FiberPulse स्वतः दोबारा नहीं पूछेगा।"],
  ["Running", "En cours", "Läuft", "En ejecución", "Em execução", "In esecuzione", "चल रहा है"],
  ["Unavailable in this build", "Indisponible dans cette version", "In dieser Version nicht verfügbar", "No disponible en esta versión", "Indisponível nesta versão", "Non disponibile in questa versione", "इस संस्करण में उपलब्ध नहीं"],
  ["Ready for provider reports", "Prêt pour les rapports au fournisseur", "Bereit für Anbieterberichte", "Listo para informes al proveedor", "Pronto para relatórios ao provedor", "Pronto per i rapporti all’operatore", "प्रदाता रिपोर्ट के लिए तैयार"],
  ["Account details not completed", "Informations du compte incomplètes", "Kontodaten unvollständig", "Datos de cuenta incompletos", "Dados da conta incompletos", "Dati account incompleti", "खाता विवरण अधूरा"],
  ["Network context, confidence, incidents and exports", "Contexte réseau, confiance, incidents et exports", "Netzwerkkontext, Vertrauen, Vorfälle und Exporte", "Contexto de red, confianza, incidencias y exportaciones", "Contexto da rede, confiança, incidentes e exportações", "Contesto di rete, affidabilità, incidenti ed esportazioni", "नेटवर्क संदर्भ, विश्वसनीयता, घटनाएँ और निर्यात"],
  ["No test", "Aucun test", "Kein Test", "Sin pruebas", "Nenhum teste", "Nessun test", "कोई परीक्षण नहीं"],
  ["Yes", "Oui", "Ja", "Sí", "Sim", "Sì", "हाँ"],
  ["No", "Non", "Nein", "No", "Não", "No", "नहीं"],
  ["Not detected", "Non détecté", "Nicht erkannt", "No detectado", "Não detectado", "Non rilevato", "नहीं मिला"],
  ["Suspected", "Suspecté", "Vermutet", "Sospechado", "Suspeito", "Sospetto", "संदिग्ध"],
  ["Collecting data", "Collecte des données", "Daten werden gesammelt", "Recopilando datos", "Coletando dados", "Raccolta dati", "डेटा एकत्र हो रहा है"],
  ["None", "Aucun", "Keine", "Ninguno", "Nenhum", "Nessuno", "कोई नहीं"],
  ["Ethernet cable", "Câble Ethernet", "Ethernet-Kabel", "Cable Ethernet", "Cabo Ethernet", "Cavo Ethernet", "ईथरनेट केबल"],
  ["Wi-Fi", "Wi-Fi", "WLAN", "Wi-Fi", "Wi-Fi", "Wi-Fi", "Wi-Fi"],
  ["Not specified", "Non précisé", "Nicht angegeben", "No especificado", "Não especificado", "Non specificato", "निर्दिष्ट नहीं"],
  ["Other", "Autre", "Andere", "Otro", "Outro", "Altro", "अन्य"],
  ["A 15-minute time bucket, approximate city area, measured speed, latency, connection type and the public details of your selected plan.", "Une tranche horaire de 15 minutes, une zone urbaine approximative, le débit, la latence, le type de connexion et les informations publiques de l’offre choisie.", "Ein 15-Minuten-Zeitfenster, ungefähres Stadtgebiet, Geschwindigkeit, Latenz, Verbindungstyp und öffentliche Tarifdaten.", "Un intervalo de 15 minutos, zona urbana aproximada, velocidad, latencia, tipo de conexión y datos públicos de la tarifa.", "Uma faixa de 15 minutos, área urbana aproximada, velocidade, latência, tipo de conexão e dados públicos do plano.", "Una fascia di 15 minuti, area urbana approssimativa, velocità, latenza, tipo di connessione e dati pubblici del piano.", "15 मिनट का समय खंड, अनुमानित शहर क्षेत्र, गति, विलंबता, कनेक्शन प्रकार और चुने गए प्लान की सार्वजनिक जानकारी।"],
  ["No name, email, account number, street address, exact IP, GPS, SSID, hostname, hardware profile or local logs are published.", "Aucun nom, e-mail, numéro de compte, adresse exacte, IP exacte, GPS, SSID, nom d’hôte, profil matériel ni journal local n’est publié.", "Name, E-Mail, Kundennummer, genaue Adresse/IP, GPS, SSID, Hostname, Hardwareprofil und lokale Protokolle werden nicht veröffentlicht.", "No se publican nombre, correo, cuenta, dirección, IP exacta, GPS, SSID, host, perfil de hardware ni registros locales.", "Nome, e-mail, conta, endereço, IP exato, GPS, SSID, host, perfil de hardware e logs locais não são publicados.", "Non vengono pubblicati nome, e-mail, account, indirizzo, IP esatto, GPS, SSID, hostname, profilo hardware o log locali.", "नाम, ईमेल, खाता संख्या, पता, सटीक IP, GPS, SSID, होस्टनाम, हार्डवेयर प्रोफ़ाइल या स्थानीय लॉग प्रकाशित नहीं होते।"],
  ["Disable sharing anytime; delivery stops and the local queue is cleared. Measurements already published remain in the anonymous statistical dataset.", "Désactivez le partage à tout moment : l’envoi s’arrête et la file locale est effacée. Les mesures déjà publiées restent dans le jeu statistique anonyme.", "Die Freigabe kann jederzeit deaktiviert werden; Übertragung und lokale Warteschlange werden beendet. Bereits veröffentlichte Messungen bleiben anonym erhalten.", "Puedes desactivar el uso compartido; se detiene el envío y se vacía la cola local. Las mediciones ya publicadas permanecen anónimas.", "Desative o compartilhamento a qualquer momento; o envio para e a fila local é limpa. Medições já publicadas permanecem anônimas.", "Puoi disattivare la condivisione; l’invio si ferma e la coda locale viene svuotata. Le misure già pubblicate restano anonime.", "साझाकरण कभी भी बंद करें; भेजना रुकता है और स्थानीय कतार साफ होती है। पहले प्रकाशित मापन अनाम आँकड़ों में रहते हैं।"],
  ["Compare your results with the offer you pay for.", "Comparez vos résultats à l’offre que vous payez.", "Vergleichen Sie Ihre Ergebnisse mit Ihrem bezahlten Tarif.", "Compara los resultados con la tarifa que pagas.", "Compare os resultados com o plano contratado.", "Confronta i risultati con il piano che paghi.", "अपने परिणामों की तुलना अपने भुगतान वाले प्लान से करें।"],
  ["Your recent checks and plan performance look normal.", "Vos vérifications récentes et les performances de votre offre semblent normales.", "Ihre letzten Prüfungen und die Tarifleistung sehen normal aus.", "Las comprobaciones recientes y el rendimiento de la tarifa son normales.", "As verificações recentes e o desempenho do plano estão normais.", "I controlli recenti e le prestazioni del piano risultano normali.", "हाल की जाँच और प्लान का प्रदर्शन सामान्य है।"],
  ["Unknown", "Inconnu", "Unbekannt", "Desconocido", "Desconhecido", "Sconosciuto", "अज्ञात"],
  ["Speed tests off", "Tests de débit désactivés", "Geschwindigkeitstests aus", "Pruebas desactivadas", "Testes desativados", "Test di velocità disattivati", "स्पीड टेस्ट बंद"],
  ["Enable it once in Settings.", "Activez-les une fois dans les Réglages.", "Einmal in den Einstellungen aktivieren.", "Actívalas una vez en Ajustes.", "Ative uma vez nas Configurações.", "Attivali una volta nelle Impostazioni.", "सेटिंग्स में एक बार सक्षम करें।"],
  ["Automatic tests are paused.", "Les tests automatiques sont en pause.", "Automatische Tests sind pausiert.", "Las pruebas automáticas están en pausa.", "Os testes automáticos estão pausados.", "I test automatici sono in pausa.", "स्वचालित परीक्षण रुके हुए हैं।"],
  ["Additional router model", "Modèle du routeur supplémentaire", "Modell des zusätzlichen Routers", "Modelo del router adicional", "Modelo do roteador adicional", "Modello router aggiuntivo", "अतिरिक्त राउटर मॉडल"],
  ["Advertised", "Annoncé", "Beworben", "Anunciado", "Anunciado", "Pubblicizzato", "विज्ञापित"],
  ["Also disconnect any system VPN profile currently active.", "Déconnectez également tout profil VPN système actif.", "Trennen Sie auch alle aktiven System-VPN-Profile.", "Desconecta también cualquier perfil VPN del sistema.", "Desconecte também qualquer perfil VPN do sistema.", "Disconnetti anche ogni profilo VPN di sistema attivo.", "सक्रिय सिस्टम VPN प्रोफ़ाइल भी बंद करें।"],
  ["Branded, readable and ready to share", "Soigné, lisible et prêt à partager", "Gestaltet, lesbar und bereit zum Teilen", "Con diseño, legible y listo para compartir", "Com identidade visual, legível e pronto para compartilhar", "Curato, leggibile e pronto da condividere", "ब्रांडेड, पठनीय और साझा करने के लिए तैयार"],
  ["Choose your plan", "Choisir votre offre", "Tarif auswählen", "Elegir tu tarifa", "Escolher seu plano", "Scegli il tuo piano", "अपना प्लान चुनें"],
  ["Complete rows for your own analysis", "Lignes complètes pour votre propre analyse", "Vollständige Zeilen für Ihre Analyse", "Filas completas para tu análisis", "Linhas completas para sua análise", "Righe complete per la tua analisi", "अपने विश्लेषण के लिए पूरी पंक्तियाँ"],
  ["Connect this Mac directly to the router supplied by your Internet provider.", "Connectez directement cet ordinateur au routeur fourni par votre opérateur.", "Verbinden Sie den Computer direkt mit dem Router Ihres Anbieters.", "Conecta el equipo directamente al router del proveedor.", "Conecte o computador diretamente ao roteador do provedor.", "Collega il computer direttamente al router dell’operatore.", "कंप्यूटर को सीधे प्रदाता के राउटर से जोड़ें।"],
  ["FiberPulse checks the active route again and refuses the test if the VPN is still detected.", "FiberPulse revérifie la route active et refuse le test si le VPN est toujours détecté.", "FiberPulse prüft die aktive Route erneut und verweigert den Test, wenn das VPN noch aktiv ist.", "FiberPulse vuelve a comprobar la ruta y rechaza la prueba si detecta la VPN.", "O FiberPulse verifica a rota novamente e recusa o teste se a VPN ainda estiver ativa.", "FiberPulse ricontrolla il percorso e rifiuta il test se la VPN è ancora attiva.", "FiberPulse सक्रिय रूट फिर जाँचता है और VPN मिलने पर परीक्षण रोकता है।"],
  ["FiberPulse never sends a complaint automatically. The .eml file includes the PDF attachment; “Open email app” fills the text only, so attach the downloaded PDF manually.", "FiberPulse n’envoie jamais de réclamation automatiquement. Le fichier .eml inclut le PDF ; « Ouvrir l’application e-mail » remplit uniquement le texte, vous devez donc joindre le PDF téléchargé manuellement.", "FiberPulse sendet Beschwerden nie automatisch. Die .eml-Datei enthält das PDF; „E-Mail-App öffnen“ füllt nur den Text, daher das PDF manuell anhängen.", "FiberPulse nunca envía una reclamación automáticamente. El .eml incluye el PDF; «Abrir correo» solo rellena el texto, por lo que debes adjuntar el PDF manualmente.", "O FiberPulse nunca envia reclamações automaticamente. O .eml inclui o PDF; “Abrir e-mail” preenche apenas o texto, então anexe o PDF manualmente.", "FiberPulse non invia mai reclami automaticamente. Il file .eml include il PDF; “Apri app e-mail” compila solo il testo, quindi allega il PDF manualmente.", "FiberPulse शिकायत स्वतः नहीं भेजता। .eml में PDF होता है; “ईमेल ऐप खोलें” केवल टेक्स्ट भरता है, इसलिए PDF स्वयं संलग्न करें।"],
  ["Mesh model", "Modèle du système mesh", "Mesh-Modell", "Modelo del sistema mesh", "Modelo do sistema mesh", "Modello sistema mesh", "मेश मॉडल"],
  ["Mesh network", "Réseau mesh", "Mesh-Netzwerk", "Red mesh", "Rede mesh", "Rete mesh", "मेश नेटवर्क"],
  ["Mixed", "Mixte", "Gemischt", "Mixta", "Mista", "Mista", "मिश्रित"],
  ["Move as close as possible to the provider router before starting the measurement.", "Rapprochez-vous autant que possible du routeur du fournisseur avant la mesure.", "Gehen Sie vor der Messung möglichst nahe an den Anbieter-Router.", "Acércate al router del proveedor antes de medir.", "Aproxime-se do roteador do provedor antes da medição.", "Avvicinati il più possibile al router prima della misurazione.", "मापन से पहले प्रदाता के राउटर के जितना संभव हो पास जाएँ।"],
  ["Open support ↗", "Ouvrir l’assistance ↗", "Support öffnen ↗", "Abrir soporte ↗", "Abrir suporte ↗", "Apri assistenza ↗", "सहायता खोलें ↗"],
  ["Provider modem in bridge + own router", "Modem fournisseur en mode bridge + routeur personnel", "Anbieter-Modem im Bridge-Modus + eigener Router", "Módem del proveedor en bridge + router propio", "Modem do provedor em bridge + roteador próprio", "Modem operatore in bridge + router proprio", "ब्रिज मोड में प्रदाता मॉडेम + अपना राउटर"],
  ["Provider router + own router", "Routeur fournisseur + routeur personnel", "Anbieter-Router + eigener Router", "Router del proveedor + router propio", "Roteador do provedor + roteador próprio", "Router operatore + router proprio", "प्रदाता राउटर + अपना राउटर"],
  ["Provider router only", "Routeur du fournisseur uniquement", "Nur Anbieter-Router", "Solo router del proveedor", "Somente roteador do provedor", "Solo router operatore", "केवल प्रदाता राउटर"],
  ["Provider support email override", "E-mail d’assistance du fournisseur", "Abweichende Support-E-Mail", "Correo de soporte del proveedor", "E-mail de suporte do provedor", "E-mail assistenza operatore", "प्रदाता सहायता ईमेल"],
  ["Provider support phone override", "Téléphone d’assistance du fournisseur", "Abweichende Support-Telefonnummer", "Teléfono de soporte del proveedor", "Telefone de suporte do provedor", "Telefono assistenza operatore", "प्रदाता सहायता फ़ोन"],
  ["Ready to review", "Prêt à vérifier", "Bereit zur Prüfung", "Listo para revisar", "Pronto para revisar", "Pronto da verificare", "समीक्षा के लिए तैयार"],
  ["Stop large downloads, cloud backups and streaming during the test.", "Arrêtez les gros téléchargements, sauvegardes cloud et streaming pendant le test.", "Stoppen Sie große Downloads, Cloud-Backups und Streaming während des Tests.", "Detén descargas grandes, copias en la nube y streaming durante la prueba.", "Pare downloads grandes, backups em nuvem e streaming durante o teste.", "Interrompi download pesanti, backup cloud e streaming durante il test.", "परीक्षण के दौरान बड़े डाउनलोड, क्लाउड बैकअप और स्ट्रीमिंग रोकें।"],
  ["Understand the conditions behind each measurement and keep evidence ready when you need to speak with your provider.", "Comprenez les conditions de chaque mesure et gardez vos preuves prêtes pour votre fournisseur.", "Verstehen Sie die Bedingungen jeder Messung und halten Sie Nachweise für den Anbieter bereit.", "Comprende las condiciones de cada medición y conserva pruebas para hablar con el proveedor.", "Entenda as condições de cada medição e mantenha evidências prontas para o provedor.", "Comprendi le condizioni di ogni misura e conserva le prove per l’operatore.", "हर मापन की परिस्थितियाँ समझें और प्रदाता से बात करने के लिए प्रमाण तैयार रखें।"],
  ["Your history stays on this device", "Votre historique reste sur cet appareil", "Ihr Verlauf bleibt auf diesem Gerät", "Tu historial permanece en este dispositivo", "Seu histórico fica neste dispositivo", "La cronologia resta su questo dispositivo", "आपका इतिहास इसी डिवाइस पर रहता है"],
  ["Your offer", "Votre offre", "Ihr Tarif", "Tu oferta", "Sua oferta", "La tua offerta", "आपका ऑफ़र"],
  ["CONNECTION", "CONNEXION", "VERBINDUNG", "CONEXIÓN", "CONEXÃO", "CONNESSIONE", "कनेक्शन"],
  ["MEASUREMENT QUALITY", "QUALITÉ DE LA MESURE", "MESSQUALITÄT", "CALIDAD DE MEDICIÓN", "QUALIDADE DA MEDIÇÃO", "QUALITÀ DELLA MISURA", "मापन गुणवत्ता"],
  ["PERSONAL REFERENCE", "RÉFÉRENCE PERSONNELLE", "PERSÖNLICHE REFERENZ", "REFERENCIA PERSONAL", "REFERÊNCIA PESSOAL", "RIFERIMENTO PERSONALE", "व्यक्तिगत संदर्भ"],
  ["SUBSCRIBED OFFER", "OFFRE SOUSCRITE", "GEBUCHTER TARIF", "TARIFA CONTRATADA", "PLANO CONTRATADO", "OFFERTA SOTTOSCRITTA", "सदस्यता वाला ऑफ़र"],
  ["OFFICIAL PROVIDER CHANNEL", "CANAL OFFICIEL DU FOURNISSEUR", "OFFIZIELLER ANBIETERKANAL", "CANAL OFICIAL DEL PROVEEDOR", "CANAL OFICIAL DO PROVEDOR", "CANALE UFFICIALE OPERATORE", "आधिकारिक प्रदाता चैनल"],
  ["LOCAL", "LOCAL", "LOKAL", "LOCAL", "LOCAL", "LOCALE", "स्थानीय"],
];

const localeIndex: Record<Locale, number> = { en: 0, fr: 1, de: 2, es: 3, "pt-BR": 4, it: 5, hi: 6 };
const dictionaries = Object.fromEntries(localeOptions.map(({ code }) => [code, new Map(rows.map(row => [row[0], row[localeIndex[code]]]))])) as Record<Locale, Map<string, string>>;

export function detectLocale(value = navigator.language): Locale {
  const language = value.toLowerCase();
  if (language.startsWith("fr")) return "fr";
  if (language.startsWith("de")) return "de";
  if (language.startsWith("es")) return "es";
  if (language.startsWith("pt")) return "pt-BR";
  if (language.startsWith("it")) return "it";
  if (language.startsWith("hi")) return "hi";
  return "en";
}

export function translate(locale: Locale, source: string) {
  if (locale === "en") return source;
  return dictionaries[locale].get(source) || source;
}

const textState = new WeakMap<Text, { source: string; rendered: string }>();
const translatedAttributes = ["aria-label", "placeholder", "title"] as const;

function translateText(locale: Locale, source: string) {
  const leading = source.match(/^\s*/)?.[0] || "";
  const trailing = source.match(/\s*$/)?.[0] || "";
  const body = source.trim();
  if (!body) return source;
  const exact = translate(locale, body);
  if (exact !== body) return leading + exact + trailing;

  // Dynamic UI strings keep their measurements while localizing their label.
  const patterns: Array<[RegExp, (match: RegExpMatchArray) => string]> = [
    [/^Last speed test: (.+)$/, m => `${translate(locale, "Last speed test")}: ${m[1]}`],
    [/^Next check: (.+)$/, m => `${translate(locale, "Next check")}: ${m[1]}`],
    [/^(\d+) tests?$/, m => `${m[1]} ${translate(locale, Number(m[1]) === 1 ? "test" : "tests")}`],
    [/^(\d+) issues?$/, m => `${m[1]} ${translate(locale, Number(m[1]) === 1 ? "issue" : "issues")}`],
    [/^Up to (.+)$/, m => `${translate(locale, "Up to")} ${m[1]}`],
    [/^Average (.+)$/, m => `${translate(locale, "Average")} ${m[1]}`],
    [/^Typically (.+)$/, m => `${translate(locale, "Typically")} ${m[1]}`],
  ];
  for (const [pattern, replacement] of patterns) {
    const match = body.match(pattern);
    if (match) return leading + replacement(match) + trailing;
  }
  return source;
}

function translateNode(root: ParentNode, locale: Locale) {
  const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
  let current: Node | null;
  while ((current = walker.nextNode())) {
    const node = current as Text;
    const parent = node.parentElement;
    if (!parent || ["SCRIPT", "STYLE", "TEXTAREA"].includes(parent.tagName)) continue;
    let state = textState.get(node);
    if (!state) state = { source: node.data, rendered: node.data };
    // If Preact changed an existing text node after a status refresh, its
    // current value is the new English source. A locale switch, by contrast,
    // leaves current equal to the last rendered translation.
    if (node.data !== state.rendered) state.source = node.data;
    state.rendered = translateText(locale, state.source);
    textState.set(node, state);
    if (node.data !== state.rendered) node.data = state.rendered;
  }
  const elements = root instanceof Element ? [root, ...root.querySelectorAll("*")] : [...root.querySelectorAll("*")];
  for (const element of elements) {
    for (const attribute of translatedAttributes) {
      const originalKey = `i18n${attribute.replace(/(^|-)([a-z])/g, (_m, _dash, letter) => letter.toUpperCase())}`;
      const html = element as HTMLElement;
      const currentValue = element.getAttribute(attribute);
      if (!currentValue) continue;
      if (!html.dataset[originalKey]) html.dataset[originalKey] = currentValue;
      element.setAttribute(attribute, translate(locale, html.dataset[originalKey] || currentValue));
    }
  }
}

export function useLocale() {
  const [locale, setLocaleState] = useState<Locale>(() => {
    const saved = localStorage.getItem("fiberpulse-locale") as Locale | null;
    return saved && localeOptions.some(option => option.code === saved) ? saved : detectLocale();
  });

  useEffect(() => {
    localStorage.setItem("fiberpulse-locale", locale);
    document.documentElement.lang = locale;
    translateNode(document.body, locale);
    const observer = new MutationObserver(mutations => {
      for (const mutation of mutations) {
        if (mutation.type === "characterData") {
          const node = mutation.target as Text;
          const state = textState.get(node);
          if (state && node.data === state.rendered) continue;
          translateNode(node.parentElement || document.body, locale);
          continue;
        }
        for (const node of mutation.addedNodes) {
          if (node instanceof Element) translateNode(node, locale);
          else if (node instanceof Text && node.parentElement) translateNode(node.parentElement, locale);
        }
      }
    });
    observer.observe(document.body, { childList: true, characterData: true, subtree: true });
    return () => observer.disconnect();
  }, [locale]);

  return { locale, setLocale: setLocaleState, t: (source: string) => translate(locale, source) };
}
