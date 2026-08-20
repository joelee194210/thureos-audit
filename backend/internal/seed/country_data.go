package seed

import (
	"time"

	"github.com/joelee/datawatch/internal/models"
)

var now = time.Now()

func c(code, code3, name, nameEN, region string, risk models.CountryRiskLevel, sources []string, notes string) models.Country {
	return models.Country{
		Code: code, Code3: code3, Name: name, NameEN: nameEN, Region: region,
		RiskLevel: risk, RiskSources: sources, Active: true, Notes: notes,
		CreatedAt: now, UpdatedAt: now, UpdatedBy: "system",
	}
}

var (
	fatf      = []string{"FATF"}
	ofac      = []string{"OFAC"}
	basel     = []string{"Basel"}
	eu        = []string{"EU"}
	fatfOfac  = []string{"FATF", "OFAC"}
	fatfOnu   = []string{"FATF", "ONU"}
	ofacOnu   = []string{"OFAC", "ONU"}
	fatfOfOnu = []string{"FATF", "OFAC", "ONU"}
	euBasel   = []string{"EU", "Basel"}
	none      = []string{}
)

// CountryData — Catalogo completo de paises con nivel de riesgo AML.
// Fuentes: FATF grey/black list, Basel AML Index, OFAC, EU, ONU.
var CountryData = []models.Country{
	// ═══════════════════════════════════════════════════════════════
	// ALTO RIESGO — Sanciones, FATF black list, conflictos armados
	// ═══════════════════════════════════════════════════════════════
	c("KP", "PRK", "Corea del Norte", "North Korea", "Asia Oriental", "high", fatfOfOnu, "Lista negra FATF. Sanciones por programa nuclear."),
	c("IR", "IRN", "Iran", "Iran", "Medio Oriente", "high", fatfOfOnu, "Lista negra FATF. Sanciones por programa nuclear y terrorismo."),
	c("MM", "MMR", "Myanmar (Birmania)", "Myanmar", "Sudeste Asiatico", "high", fatfOfac, "Lista negra FATF desde 2022. Golpe militar."),
	c("SY", "SYR", "Siria", "Syria", "Medio Oriente", "high", []string{"FATF", "OFAC", "EU", "ONU"}, "Sanciones multiples. Conflicto armado."),
	c("AF", "AFG", "Afganistan", "Afghanistan", "Asia Central", "high", fatfOfOnu, "Regimen Taliban. Sanciones internacionales."),
	c("RU", "RUS", "Rusia", "Russia", "Europa Oriental", "high", []string{"OFAC", "EU"}, "Sanciones por conflicto en Ucrania desde 2022."),
	c("BY", "BLR", "Bielorrusia", "Belarus", "Europa Oriental", "high", []string{"OFAC", "EU"}, "Sanciones asociadas al conflicto Rusia-Ucrania."),
	c("CU", "CUB", "Cuba", "Cuba", "Caribe", "high", ofac, "Embargo comercial EEUU. Lista OFAC."),
	c("VE", "VEN", "Venezuela", "Venezuela", "Sudamerica", "high", fatfOfac, "Sanciones OFAC. Alto riesgo corrupcion y narcotrafico."),
	c("YE", "YEM", "Yemen", "Yemen", "Medio Oriente", "high", fatfOnu, "Conflicto armado. Financiamiento al terrorismo."),
	c("SD", "SDN", "Sudan", "Sudan", "Africa Oriental", "high", ofacOnu, "Conflicto interno. Sanciones internacionales."),
	c("SS", "SSD", "Sudan del Sur", "South Sudan", "Africa Oriental", "high", ofacOnu, "Conflicto armado y sanciones."),
	c("SO", "SOM", "Somalia", "Somalia", "Africa Oriental", "high", fatfOnu, "Estado fallido. Alto riesgo terrorismo."),
	c("LY", "LBY", "Libia", "Libya", "Africa del Norte", "high", []string{"ONU", "EU"}, "Inestabilidad politica."),
	c("CD", "COD", "R.D. del Congo", "DR Congo", "Africa Central", "high", []string{"ONU"}, "Conflicto armado. Minerales de conflicto."),
	c("CF", "CAF", "Rep. Centroafricana", "Central African Republic", "Africa Central", "high", []string{"ONU"}, "Conflicto armado."),
	c("LB", "LBN", "Libano", "Lebanon", "Medio Oriente", "high", []string{"Basel", "FATF"}, "Crisis financiera. Alto riesgo ML."),
	c("IQ", "IRQ", "Irak", "Iraq", "Medio Oriente", "high", []string{"Basel", "FATF"}, "Inestabilidad. Financiamiento al terrorismo."),
	c("ER", "ERI", "Eritrea", "Eritrea", "Africa Oriental", "high", []string{"ONU"}, "Sanciones ONU. Regimen autoritario."),

	// ═══════════════════════════════════════════════════════════════
	// RIESGO MEDIO — FATF grey list, Basel alto, offshore
	// ═══════════════════════════════════════════════════════════════
	c("BF", "BFA", "Burkina Faso", "Burkina Faso", "Africa Occidental", "medium", fatf, "Lista gris FATF."),
	c("CM", "CMR", "Camerun", "Cameroon", "Africa Central", "medium", fatf, "Lista gris FATF."),
	c("HR", "HRV", "Croacia", "Croatia", "Europa", "medium", fatf, "Lista gris FATF."),
	c("HT", "HTI", "Haiti", "Haiti", "Caribe", "medium", fatf, "Lista gris FATF. Crisis humanitaria."),
	c("KE", "KEN", "Kenia", "Kenya", "Africa Oriental", "medium", fatf, "Lista gris FATF."),
	c("ML", "MLI", "Mali", "Mali", "Africa Occidental", "medium", fatfOnu, "Lista gris FATF. Inestabilidad."),
	c("MZ", "MOZ", "Mozambique", "Mozambique", "Africa Oriental", "medium", fatf, "Lista gris FATF."),
	c("NG", "NGA", "Nigeria", "Nigeria", "Africa Occidental", "medium", fatf, "Lista gris FATF. Riesgo fraude."),
	c("PH", "PHL", "Filipinas", "Philippines", "Sudeste Asiatico", "medium", fatf, "Lista gris FATF."),
	c("ZA", "ZAF", "Sudafrica", "South Africa", "Africa Austral", "medium", fatf, "Lista gris FATF."),
	c("TZ", "TZA", "Tanzania", "Tanzania", "Africa Oriental", "medium", fatf, "Lista gris FATF."),
	c("VN", "VNM", "Vietnam", "Vietnam", "Sudeste Asiatico", "medium", fatf, "Lista gris FATF."),
	c("LA", "LAO", "Laos", "Laos", "Sudeste Asiatico", "medium", basel, "Alto indice Basel AML."),
	c("KH", "KHM", "Camboya", "Cambodia", "Sudeste Asiatico", "medium", []string{"Basel", "FATF"}, "Casinos y lavado transnacional."),
	c("PK", "PAK", "Pakistan", "Pakistan", "Asia del Sur", "medium", []string{"Basel", "FATF"}, "Financiamiento terrorismo."),
	c("BD", "BGD", "Bangladesh", "Bangladesh", "Asia del Sur", "medium", basel, "Alto volumen remesas. Trade-based ML."),
	c("NI", "NIC", "Nicaragua", "Nicaragua", "Centroamerica", "medium", ofac, "Sanciones selectivas OFAC."),
	c("PA", "PAN", "Panama", "Panama", "Centroamerica", "medium", euBasel, "Lista EU jurisdicciones no cooperativas."),
	c("VG", "VGB", "Islas Virgenes Britanicas", "British Virgin Islands", "Caribe", "medium", basel, "Centro financiero offshore."),
	c("KY", "CYM", "Islas Caiman", "Cayman Islands", "Caribe", "medium", basel, "Principal centro offshore."),
	c("BZ", "BLZ", "Belice", "Belize", "Centroamerica", "medium", basel, "Servicios financieros offshore."),
	c("GH", "GHA", "Ghana", "Ghana", "Africa Occidental", "medium", basel, "Indice Basel alto. Fraude cibernetico."),
	c("SN", "SEN", "Senegal", "Senegal", "Africa Occidental", "medium", fatf, "Lista gris FATF."),
	c("UG", "UGA", "Uganda", "Uganda", "Africa Oriental", "medium", basel, "Indice Basel alto."),
	c("ZW", "ZWE", "Zimbabue", "Zimbabwe", "Africa Austral", "medium", ofac, "Sanciones selectivas."),
	c("ET", "ETH", "Etiopia", "Ethiopia", "Africa Oriental", "medium", basel, "Conflicto en Tigray."),
	c("NE", "NER", "Niger", "Niger", "Africa Occidental", "medium", fatf, "Riesgo terrorismo Sahel."),
	c("TD", "TCD", "Chad", "Chad", "Africa Central", "medium", basel, "Inestabilidad."),
	c("MG", "MDG", "Madagascar", "Madagascar", "Africa Oriental", "medium", basel, "Indice Basel alto."),
	c("TJ", "TJK", "Tayikistan", "Tajikistan", "Asia Central", "medium", basel, "Corredor narcotrafico."),
	c("TM", "TKM", "Turkmenistan", "Turkmenistan", "Asia Central", "medium", basel, "Opacidad financiera."),
	c("UZ", "UZB", "Uzbekistan", "Uzbekistan", "Asia Central", "medium", basel, "Mejorando marco AML."),
	c("KG", "KGZ", "Kirguistan", "Kyrgyzstan", "Asia Central", "medium", basel, "Indice Basel medio-alto."),
	c("AZ", "AZE", "Azerbaiyan", "Azerbaijan", "Caucaso", "medium", basel, "Riesgo corrupcion."),
	c("AM", "ARM", "Armenia", "Armenia", "Caucaso", "medium", basel, "Riesgo medio."),
	c("GE", "GEO", "Georgia", "Georgia", "Caucaso", "medium", basel, "Mejorando marco regulatorio."),
	c("MN", "MNG", "Mongolia", "Mongolia", "Asia Oriental", "medium", fatf, "Lista gris FATF."),
	c("LK", "LKA", "Sri Lanka", "Sri Lanka", "Asia del Sur", "medium", basel, "Crisis economica 2022."),
	c("NP", "NPL", "Nepal", "Nepal", "Asia del Sur", "medium", basel, "Remesas y riesgo fronterizo."),
	c("BI", "BDI", "Burundi", "Burundi", "Africa Oriental", "medium", basel, "Inestabilidad politica."),
	c("RW", "RWA", "Ruanda", "Rwanda", "Africa Oriental", "medium", basel, "Indice Basel medio."),
	c("GN", "GIN", "Guinea", "Guinea", "Africa Occidental", "medium", basel, "Indice Basel alto."),
	c("SL", "SLE", "Sierra Leona", "Sierra Leone", "Africa Occidental", "medium", basel, "Post conflicto. Diamantes."),
	c("LR", "LBR", "Liberia", "Liberia", "Africa Occidental", "medium", basel, "Post conflicto. Banderas de conveniencia."),
	c("MR", "MRT", "Mauritania", "Mauritania", "Africa Occidental", "medium", basel, "Riesgo terrorismo."),
	c("DJ", "DJI", "Yibuti", "Djibouti", "Africa Oriental", "medium", basel, "Corredor comercial."),
	c("BA", "BIH", "Bosnia y Herzegovina", "Bosnia and Herzegovina", "Europa", "medium", fatf, "Lista gris FATF."),
	c("AL", "ALB", "Albania", "Albania", "Europa", "medium", basel, "Indice Basel medio. Crimen organizado."),
	c("UA", "UKR", "Ucrania", "Ukraine", "Europa Oriental", "medium", basel, "Conflicto armado activo."),
	c("TR", "TUR", "Turquia", "Turkey", "Europa/Asia", "medium", fatf, "Lista gris FATF. Riesgo ML."),

	// ═══════════════════════════════════════════════════════════════
	// BAJO RIESGO — Paises con marcos AML robustos
	// ═══════════════════════════════════════════════════════════════

	// Americas
	c("US", "USA", "Estados Unidos", "United States", "Norteamerica", "low", none, ""),
	c("CA", "CAN", "Canada", "Canada", "Norteamerica", "low", none, ""),
	c("MX", "MEX", "Mexico", "Mexico", "Norteamerica", "low", none, ""),
	c("GT", "GTM", "Guatemala", "Guatemala", "Centroamerica", "low", none, ""),
	c("HN", "HND", "Honduras", "Honduras", "Centroamerica", "low", none, ""),
	c("SV", "SLV", "El Salvador", "El Salvador", "Centroamerica", "low", none, ""),
	c("CR", "CRI", "Costa Rica", "Costa Rica", "Centroamerica", "low", none, ""),
	c("CO", "COL", "Colombia", "Colombia", "Sudamerica", "low", none, ""),
	c("EC", "ECU", "Ecuador", "Ecuador", "Sudamerica", "low", none, ""),
	c("PE", "PER", "Peru", "Peru", "Sudamerica", "low", none, ""),
	c("BR", "BRA", "Brasil", "Brazil", "Sudamerica", "low", none, ""),
	c("CL", "CHL", "Chile", "Chile", "Sudamerica", "low", none, ""),
	c("AR", "ARG", "Argentina", "Argentina", "Sudamerica", "low", none, ""),
	c("UY", "URY", "Uruguay", "Uruguay", "Sudamerica", "low", none, ""),
	c("PY", "PRY", "Paraguay", "Paraguay", "Sudamerica", "low", none, ""),
	c("BO", "BOL", "Bolivia", "Bolivia", "Sudamerica", "low", none, ""),
	c("GY", "GUY", "Guyana", "Guyana", "Sudamerica", "low", none, ""),
	c("SR", "SUR", "Surinam", "Suriname", "Sudamerica", "low", none, ""),
	c("DO", "DOM", "Rep. Dominicana", "Dominican Republic", "Caribe", "low", none, ""),
	c("JM", "JAM", "Jamaica", "Jamaica", "Caribe", "low", none, ""),
	c("TT", "TTO", "Trinidad y Tobago", "Trinidad and Tobago", "Caribe", "low", none, ""),
	c("BB", "BRB", "Barbados", "Barbados", "Caribe", "low", none, ""),
	c("BS", "BHS", "Bahamas", "Bahamas", "Caribe", "low", none, ""),
	c("AG", "ATG", "Antigua y Barbuda", "Antigua and Barbuda", "Caribe", "low", none, ""),
	c("DM", "DMA", "Dominica", "Dominica", "Caribe", "low", none, ""),
	c("GD", "GRD", "Granada", "Grenada", "Caribe", "low", none, ""),
	c("KN", "KNA", "San Cristobal y Nieves", "Saint Kitts and Nevis", "Caribe", "low", none, ""),
	c("LC", "LCA", "Santa Lucia", "Saint Lucia", "Caribe", "low", none, ""),
	c("VC", "VCT", "San Vicente y Granadinas", "Saint Vincent", "Caribe", "low", none, ""),

	// Europa Occidental y Central
	c("GB", "GBR", "Reino Unido", "United Kingdom", "Europa Occidental", "low", none, ""),
	c("DE", "DEU", "Alemania", "Germany", "Europa Occidental", "low", none, ""),
	c("FR", "FRA", "Francia", "France", "Europa Occidental", "low", none, ""),
	c("ES", "ESP", "Espana", "Spain", "Europa Occidental", "low", none, ""),
	c("IT", "ITA", "Italia", "Italy", "Europa Occidental", "low", none, ""),
	c("PT", "PRT", "Portugal", "Portugal", "Europa Occidental", "low", none, ""),
	c("NL", "NLD", "Paises Bajos", "Netherlands", "Europa Occidental", "low", none, ""),
	c("BE", "BEL", "Belgica", "Belgium", "Europa Occidental", "low", none, ""),
	c("LU", "LUX", "Luxemburgo", "Luxembourg", "Europa Occidental", "low", none, ""),
	c("CH", "CHE", "Suiza", "Switzerland", "Europa Occidental", "low", none, ""),
	c("AT", "AUT", "Austria", "Austria", "Europa Central", "low", none, ""),
	c("IE", "IRL", "Irlanda", "Ireland", "Europa Occidental", "low", none, ""),
	c("SE", "SWE", "Suecia", "Sweden", "Europa del Norte", "low", none, ""),
	c("NO", "NOR", "Noruega", "Norway", "Europa del Norte", "low", none, ""),
	c("DK", "DNK", "Dinamarca", "Denmark", "Europa del Norte", "low", none, ""),
	c("FI", "FIN", "Finlandia", "Finland", "Europa del Norte", "low", none, ""),
	c("IS", "ISL", "Islandia", "Iceland", "Europa del Norte", "low", none, ""),
	c("PL", "POL", "Polonia", "Poland", "Europa Central", "low", none, ""),
	c("CZ", "CZE", "Rep. Checa", "Czech Republic", "Europa Central", "low", none, ""),
	c("SK", "SVK", "Eslovaquia", "Slovakia", "Europa Central", "low", none, ""),
	c("HU", "HUN", "Hungria", "Hungary", "Europa Central", "low", none, ""),
	c("RO", "ROU", "Rumania", "Romania", "Europa Oriental", "low", none, ""),
	c("BG", "BGR", "Bulgaria", "Bulgaria", "Europa Oriental", "low", none, ""),
	c("GR", "GRC", "Grecia", "Greece", "Europa del Sur", "low", none, ""),
	c("SI", "SVN", "Eslovenia", "Slovenia", "Europa Central", "low", none, ""),
	c("EE", "EST", "Estonia", "Estonia", "Europa del Norte", "low", none, ""),
	c("LV", "LVA", "Letonia", "Latvia", "Europa del Norte", "low", none, ""),
	c("LT", "LTU", "Lituania", "Lithuania", "Europa del Norte", "low", none, ""),
	c("MT", "MLT", "Malta", "Malta", "Europa del Sur", "low", none, ""),
	c("CY", "CYP", "Chipre", "Cyprus", "Europa del Sur", "low", none, ""),
	c("RS", "SRB", "Serbia", "Serbia", "Europa", "low", none, ""),
	c("ME", "MNE", "Montenegro", "Montenegro", "Europa", "low", none, ""),
	c("MK", "MKD", "Macedonia del Norte", "North Macedonia", "Europa", "low", none, ""),
	c("MD", "MDA", "Moldavia", "Moldova", "Europa Oriental", "low", none, ""),
	c("AD", "AND", "Andorra", "Andorra", "Europa Occidental", "low", none, ""),
	c("MC", "MCO", "Monaco", "Monaco", "Europa Occidental", "low", none, ""),
	c("SM", "SMR", "San Marino", "San Marino", "Europa del Sur", "low", none, ""),
	c("LI", "LIE", "Liechtenstein", "Liechtenstein", "Europa Occidental", "low", none, ""),
	c("VA", "VAT", "Ciudad del Vaticano", "Vatican City", "Europa del Sur", "low", none, ""),
	c("XK", "XKX", "Kosovo", "Kosovo", "Europa", "low", none, ""),

	// Asia y Oceania
	c("CN", "CHN", "China", "China", "Asia Oriental", "low", none, ""),
	c("JP", "JPN", "Japon", "Japan", "Asia Oriental", "low", none, ""),
	c("KR", "KOR", "Corea del Sur", "South Korea", "Asia Oriental", "low", none, ""),
	c("TW", "TWN", "Taiwan", "Taiwan", "Asia Oriental", "low", none, ""),
	c("HK", "HKG", "Hong Kong", "Hong Kong", "Asia Oriental", "low", none, ""),
	c("MO", "MAC", "Macao", "Macao", "Asia Oriental", "low", none, ""),
	c("SG", "SGP", "Singapur", "Singapore", "Sudeste Asiatico", "low", none, ""),
	c("MY", "MYS", "Malasia", "Malaysia", "Sudeste Asiatico", "low", none, ""),
	c("TH", "THA", "Tailandia", "Thailand", "Sudeste Asiatico", "low", none, ""),
	c("ID", "IDN", "Indonesia", "Indonesia", "Sudeste Asiatico", "low", none, ""),
	c("BN", "BRN", "Brunei", "Brunei", "Sudeste Asiatico", "low", none, ""),
	c("TL", "TLS", "Timor Oriental", "Timor-Leste", "Sudeste Asiatico", "low", none, ""),
	c("IN", "IND", "India", "India", "Asia del Sur", "low", none, ""),
	c("BT", "BTN", "Butan", "Bhutan", "Asia del Sur", "low", none, ""),
	c("MV", "MDV", "Maldivas", "Maldives", "Asia del Sur", "low", none, ""),
	c("AU", "AUS", "Australia", "Australia", "Oceania", "low", none, ""),
	c("NZ", "NZL", "Nueva Zelanda", "New Zealand", "Oceania", "low", none, ""),
	c("FJ", "FJI", "Fiyi", "Fiji", "Oceania", "low", none, ""),
	c("PG", "PNG", "Papua Nueva Guinea", "Papua New Guinea", "Oceania", "low", none, ""),
	c("WS", "WSM", "Samoa", "Samoa", "Oceania", "low", none, ""),
	c("TO", "TON", "Tonga", "Tonga", "Oceania", "low", none, ""),
	c("VU", "VUT", "Vanuatu", "Vanuatu", "Oceania", "low", none, ""),
	c("KZ", "KAZ", "Kazajistan", "Kazakhstan", "Asia Central", "low", none, ""),

	// Medio Oriente
	c("AE", "ARE", "Emiratos Arabes Unidos", "United Arab Emirates", "Medio Oriente", "low", none, ""),
	c("SA", "SAU", "Arabia Saudita", "Saudi Arabia", "Medio Oriente", "low", none, ""),
	c("QA", "QAT", "Catar", "Qatar", "Medio Oriente", "low", none, ""),
	c("KW", "KWT", "Kuwait", "Kuwait", "Medio Oriente", "low", none, ""),
	c("BH", "BHR", "Barein", "Bahrain", "Medio Oriente", "low", none, ""),
	c("OM", "OMN", "Oman", "Oman", "Medio Oriente", "low", none, ""),
	c("JO", "JOR", "Jordania", "Jordan", "Medio Oriente", "low", none, ""),
	c("IL", "ISR", "Israel", "Israel", "Medio Oriente", "low", none, ""),
	c("PS", "PSE", "Palestina", "Palestine", "Medio Oriente", "low", none, ""),

	// Africa (low risk)
	c("MA", "MAR", "Marruecos", "Morocco", "Africa del Norte", "low", none, ""),
	c("TN", "TUN", "Tunez", "Tunisia", "Africa del Norte", "low", none, ""),
	c("DZ", "DZA", "Argelia", "Algeria", "Africa del Norte", "low", none, ""),
	c("EG", "EGY", "Egipto", "Egypt", "Africa del Norte", "low", none, ""),
	c("MU", "MUS", "Mauricio", "Mauritius", "Africa Oriental", "low", none, ""),
	c("BW", "BWA", "Botsuana", "Botswana", "Africa Austral", "low", none, ""),
	c("NA", "NAM", "Namibia", "Namibia", "Africa Austral", "low", none, ""),
	c("SC", "SYC", "Seychelles", "Seychelles", "Africa Oriental", "low", none, ""),
	c("CV", "CPV", "Cabo Verde", "Cape Verde", "Africa Occidental", "low", none, ""),
	c("CI", "CIV", "Costa de Marfil", "Ivory Coast", "Africa Occidental", "low", none, ""),
	c("TG", "TGO", "Togo", "Togo", "Africa Occidental", "low", none, ""),
	c("BJ", "BEN", "Benin", "Benin", "Africa Occidental", "low", none, ""),
	c("GM", "GMB", "Gambia", "Gambia", "Africa Occidental", "low", none, ""),
	c("GW", "GNB", "Guinea-Bisau", "Guinea-Bissau", "Africa Occidental", "low", none, ""),
	c("GA", "GAB", "Gabon", "Gabon", "Africa Central", "low", none, ""),
	c("CG", "COG", "Rep. del Congo", "Republic of Congo", "Africa Central", "low", none, ""),
	c("GQ", "GNQ", "Guinea Ecuatorial", "Equatorial Guinea", "Africa Central", "low", none, ""),
	c("AO", "AGO", "Angola", "Angola", "Africa Austral", "low", none, ""),
	c("ZM", "ZMB", "Zambia", "Zambia", "Africa Austral", "low", none, ""),
	c("MW", "MWI", "Malaui", "Malawi", "Africa Oriental", "low", none, ""),
	c("LS", "LSO", "Lesoto", "Lesotho", "Africa Austral", "low", none, ""),
	c("SZ", "SWZ", "Esuatini", "Eswatini", "Africa Austral", "low", none, ""),
	c("KM", "COM", "Comoras", "Comoros", "Africa Oriental", "low", none, ""),
	c("ST", "STP", "Santo Tome y Principe", "Sao Tome and Principe", "Africa Central", "low", none, ""),
}
