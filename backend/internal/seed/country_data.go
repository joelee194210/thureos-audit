package seed

import (
	"time"

	"github.com/thureos/compliance/internal/models"
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
	c("IR", "IRN", "Irán", "Iran", "Medio Oriente", "high", fatfOfOnu, "Lista negra FATF. Sanciones por programa nuclear y terrorismo."),
	c("MM", "MMR", "Myanmar (Birmania)", "Myanmar", "Sudeste Asiático", "high", fatfOfac, "Lista negra FATF desde 2022. Golpe militar."),
	c("SY", "SYR", "Siria", "Syria", "Medio Oriente", "high", []string{"FATF", "OFAC", "EU", "ONU"}, "Sanciones multiples. Conflicto armado."),
	c("AF", "AFG", "Afganistán", "Afghanistan", "Asia Central", "high", fatfOfOnu, "Regimen Taliban. Sanciones internacionales."),
	c("RU", "RUS", "Rusia", "Russia", "Europa Oriental", "high", []string{"OFAC", "EU"}, "Sanciones por conflicto en Ucrania desde 2022."),
	c("BY", "BLR", "Bielorrusia", "Belarus", "Europa Oriental", "high", []string{"OFAC", "EU"}, "Sanciones asociadas al conflicto Rusia-Ucrania."),
	c("CU", "CUB", "Cuba", "Cuba", "Caribe", "high", ofac, "Embargo comercial EEUU. Lista OFAC."),
	c("VE", "VEN", "Venezuela", "Venezuela", "Sudamérica", "high", fatfOfac, "Sanciones OFAC. Alto riesgo corrupcion y narcotrafico."),
	c("YE", "YEM", "Yemen", "Yemen", "Medio Oriente", "high", fatfOnu, "Conflicto armado. Financiamiento al terrorismo."),
	c("SD", "SDN", "Sudán", "Sudan", "África Oriental", "high", ofacOnu, "Conflicto interno. Sanciones internacionales."),
	c("SS", "SSD", "Sudán del Sur", "South Sudan", "África Oriental", "high", ofacOnu, "Conflicto armado y sanciones."),
	c("SO", "SOM", "Somalia", "Somalia", "África Oriental", "high", fatfOnu, "Estado fallido. Alto riesgo terrorismo."),
	c("LY", "LBY", "Libia", "Libya", "África del Norte", "high", []string{"ONU", "EU"}, "Inestabilidad politica."),
	c("CD", "COD", "R.D. del Congo", "DR Congo", "África Central", "high", []string{"ONU"}, "Conflicto armado. Minerales de conflicto."),
	c("CF", "CAF", "Rep. Centroafricana", "Central African Republic", "África Central", "high", []string{"ONU"}, "Conflicto armado."),
	c("LB", "LBN", "Líbano", "Lebanon", "Medio Oriente", "high", []string{"Basel", "FATF"}, "Crisis financiera. Alto riesgo ML."),
	c("IQ", "IRQ", "Irak", "Iraq", "Medio Oriente", "high", []string{"Basel", "FATF"}, "Inestabilidad. Financiamiento al terrorismo."),
	c("ER", "ERI", "Eritrea", "Eritrea", "África Oriental", "high", []string{"ONU"}, "Sanciones ONU. Regimen autoritario."),

	// ═══════════════════════════════════════════════════════════════
	// RIESGO MEDIO — FATF grey list, Basel alto, offshore
	// ═══════════════════════════════════════════════════════════════
	c("BF", "BFA", "Burkina Faso", "Burkina Faso", "África Occidental", "medium", fatf, "Lista gris FATF."),
	c("CM", "CMR", "Camerún", "Cameroon", "África Central", "medium", fatf, "Lista gris FATF."),
	c("HR", "HRV", "Croacia", "Croatia", "Europa", "medium", fatf, "Lista gris FATF."),
	c("HT", "HTI", "Haití", "Haiti", "Caribe", "medium", fatf, "Lista gris FATF. Crisis humanitaria."),
	c("KE", "KEN", "Kenia", "Kenya", "África Oriental", "medium", fatf, "Lista gris FATF."),
	c("ML", "MLI", "Malí", "Mali", "África Occidental", "medium", fatfOnu, "Lista gris FATF. Inestabilidad."),
	c("MZ", "MOZ", "Mozambique", "Mozambique", "África Oriental", "medium", fatf, "Lista gris FATF."),
	c("NG", "NGA", "Nigeria", "Nigeria", "África Occidental", "medium", fatf, "Lista gris FATF. Riesgo fraude."),
	c("PH", "PHL", "Filipinas", "Philippines", "Sudeste Asiático", "medium", fatf, "Lista gris FATF."),
	c("ZA", "ZAF", "Sudáfrica", "South Africa", "África Austral", "medium", fatf, "Lista gris FATF."),
	c("TZ", "TZA", "Tanzania", "Tanzania", "África Oriental", "medium", fatf, "Lista gris FATF."),
	c("VN", "VNM", "Vietnam", "Vietnam", "Sudeste Asiático", "medium", fatf, "Lista gris FATF."),
	c("LA", "LAO", "Laos", "Laos", "Sudeste Asiático", "medium", basel, "Alto indice Basel AML."),
	c("KH", "KHM", "Camboya", "Cambodia", "Sudeste Asiático", "medium", []string{"Basel", "FATF"}, "Casinos y lavado transnacional."),
	c("PK", "PAK", "Pakistán", "Pakistan", "Asia del Sur", "medium", []string{"Basel", "FATF"}, "Financiamiento terrorismo."),
	c("BD", "BGD", "Bangladesh", "Bangladesh", "Asia del Sur", "medium", basel, "Alto volumen remesas. Trade-based ML."),
	c("NI", "NIC", "Nicaragua", "Nicaragua", "Centroamérica", "medium", ofac, "Sanciones selectivas OFAC."),
	c("PA", "PAN", "Panamá", "Panama", "Centroamérica", "medium", euBasel, "Lista EU jurisdicciones no cooperativas."),
	c("VG", "VGB", "Islas Vírgenes Británicas", "British Virgin Islands", "Caribe", "medium", basel, "Centro financiero offshore."),
	c("KY", "CYM", "Islas Caimán", "Cayman Islands", "Caribe", "medium", basel, "Principal centro offshore."),
	c("BZ", "BLZ", "Belice", "Belize", "Centroamérica", "medium", basel, "Servicios financieros offshore."),
	c("GH", "GHA", "Ghana", "Ghana", "África Occidental", "medium", basel, "Indice Basel alto. Fraude cibernetico."),
	c("SN", "SEN", "Senegal", "Senegal", "África Occidental", "medium", fatf, "Lista gris FATF."),
	c("UG", "UGA", "Uganda", "Uganda", "África Oriental", "medium", basel, "Indice Basel alto."),
	c("ZW", "ZWE", "Zimbabue", "Zimbabwe", "África Austral", "medium", ofac, "Sanciones selectivas."),
	c("ET", "ETH", "Etiopía", "Ethiopia", "África Oriental", "medium", basel, "Conflicto en Tigray."),
	c("NE", "NER", "Níger", "Niger", "África Occidental", "medium", fatf, "Riesgo terrorismo Sahel."),
	c("TD", "TCD", "Chad", "Chad", "África Central", "medium", basel, "Inestabilidad."),
	c("MG", "MDG", "Madagascar", "Madagascar", "África Oriental", "medium", basel, "Indice Basel alto."),
	c("TJ", "TJK", "Tayikistán", "Tajikistan", "Asia Central", "medium", basel, "Corredor narcotrafico."),
	c("TM", "TKM", "Turkmenistán", "Turkmenistan", "Asia Central", "medium", basel, "Opacidad financiera."),
	c("UZ", "UZB", "Uzbekistán", "Uzbekistan", "Asia Central", "medium", basel, "Mejorando marco AML."),
	c("KG", "KGZ", "Kirguistán", "Kyrgyzstan", "Asia Central", "medium", basel, "Indice Basel medio-alto."),
	c("AZ", "AZE", "Azerbaiyán", "Azerbaijan", "Cáucaso", "medium", basel, "Riesgo corrupcion."),
	c("AM", "ARM", "Armenia", "Armenia", "Cáucaso", "medium", basel, "Riesgo medio."),
	c("GE", "GEO", "Georgia", "Georgia", "Cáucaso", "medium", basel, "Mejorando marco regulatorio."),
	c("MN", "MNG", "Mongolia", "Mongolia", "Asia Oriental", "medium", fatf, "Lista gris FATF."),
	c("LK", "LKA", "Sri Lanka", "Sri Lanka", "Asia del Sur", "medium", basel, "Crisis economica 2022."),
	c("NP", "NPL", "Nepal", "Nepal", "Asia del Sur", "medium", basel, "Remesas y riesgo fronterizo."),
	c("BI", "BDI", "Burundi", "Burundi", "África Oriental", "medium", basel, "Inestabilidad politica."),
	c("RW", "RWA", "Ruanda", "Rwanda", "África Oriental", "medium", basel, "Indice Basel medio."),
	c("GN", "GIN", "Guinea", "Guinea", "África Occidental", "medium", basel, "Indice Basel alto."),
	c("SL", "SLE", "Sierra Leona", "Sierra Leone", "África Occidental", "medium", basel, "Post conflicto. Diamantes."),
	c("LR", "LBR", "Liberia", "Liberia", "África Occidental", "medium", basel, "Post conflicto. Banderas de conveniencia."),
	c("MR", "MRT", "Mauritania", "Mauritania", "África Occidental", "medium", basel, "Riesgo terrorismo."),
	c("DJ", "DJI", "Yibuti", "Djibouti", "África Oriental", "medium", basel, "Corredor comercial."),
	c("BA", "BIH", "Bosnia y Herzegovina", "Bosnia and Herzegovina", "Europa", "medium", fatf, "Lista gris FATF."),
	c("AL", "ALB", "Albania", "Albania", "Europa", "medium", basel, "Indice Basel medio. Crimen organizado."),
	c("UA", "UKR", "Ucrania", "Ukraine", "Europa Oriental", "medium", basel, "Conflicto armado activo."),
	c("TR", "TUR", "Turquía", "Turkey", "Europa/Asia", "medium", fatf, "Lista gris FATF. Riesgo ML."),

	// ═══════════════════════════════════════════════════════════════
	// BAJO RIESGO — Paises con marcos AML robustos
	// ═══════════════════════════════════════════════════════════════

	// Americas
	c("US", "USA", "Estados Unidos", "United States", "Norteamérica", "low", none, ""),
	c("CA", "CAN", "Canadá", "Canada", "Norteamérica", "low", none, ""),
	c("MX", "MEX", "México", "Mexico", "Norteamérica", "low", none, ""),
	c("GT", "GTM", "Guatemala", "Guatemala", "Centroamérica", "low", none, ""),
	c("HN", "HND", "Honduras", "Honduras", "Centroamérica", "low", none, ""),
	c("SV", "SLV", "El Salvador", "El Salvador", "Centroamérica", "low", none, ""),
	c("CR", "CRI", "Costa Rica", "Costa Rica", "Centroamérica", "low", none, ""),
	c("CO", "COL", "Colombia", "Colombia", "Sudamérica", "low", none, ""),
	c("EC", "ECU", "Ecuador", "Ecuador", "Sudamérica", "low", none, ""),
	c("PE", "PER", "Perú", "Peru", "Sudamérica", "low", none, ""),
	c("BR", "BRA", "Brasil", "Brazil", "Sudamérica", "low", none, ""),
	c("CL", "CHL", "Chile", "Chile", "Sudamérica", "low", none, ""),
	c("AR", "ARG", "Argentina", "Argentina", "Sudamérica", "low", none, ""),
	c("UY", "URY", "Uruguay", "Uruguay", "Sudamérica", "low", none, ""),
	c("PY", "PRY", "Paraguay", "Paraguay", "Sudamérica", "low", none, ""),
	c("BO", "BOL", "Bolivia", "Bolivia", "Sudamérica", "low", none, ""),
	c("GY", "GUY", "Guyana", "Guyana", "Sudamérica", "low", none, ""),
	c("SR", "SUR", "Surinam", "Suriname", "Sudamérica", "low", none, ""),
	c("DO", "DOM", "Rep. Dominicana", "Dominican Republic", "Caribe", "low", none, ""),
	c("JM", "JAM", "Jamaica", "Jamaica", "Caribe", "low", none, ""),
	c("TT", "TTO", "Trinidad y Tobago", "Trinidad and Tobago", "Caribe", "low", none, ""),
	c("BB", "BRB", "Barbados", "Barbados", "Caribe", "low", none, ""),
	c("BS", "BHS", "Bahamas", "Bahamas", "Caribe", "low", none, ""),
	c("AG", "ATG", "Antigua y Barbuda", "Antigua and Barbuda", "Caribe", "low", none, ""),
	c("DM", "DMA", "Dominica", "Dominica", "Caribe", "low", none, ""),
	c("GD", "GRD", "Granada", "Grenada", "Caribe", "low", none, ""),
	c("KN", "KNA", "San Cristóbal y Nieves", "Saint Kitts and Nevis", "Caribe", "low", none, ""),
	c("LC", "LCA", "Santa Lucía", "Saint Lucia", "Caribe", "low", none, ""),
	c("VC", "VCT", "San Vicente y Granadinas", "Saint Vincent", "Caribe", "low", none, ""),

	// Europa Occidental y Central
	c("GB", "GBR", "Reino Unido", "United Kingdom", "Europa Occidental", "low", none, ""),
	c("DE", "DEU", "Alemania", "Germany", "Europa Occidental", "low", none, ""),
	c("FR", "FRA", "Francia", "France", "Europa Occidental", "low", none, ""),
	c("ES", "ESP", "España", "Spain", "Europa Occidental", "low", none, ""),
	c("IT", "ITA", "Italia", "Italy", "Europa Occidental", "low", none, ""),
	c("PT", "PRT", "Portugal", "Portugal", "Europa Occidental", "low", none, ""),
	c("NL", "NLD", "Países Bajos", "Netherlands", "Europa Occidental", "low", none, ""),
	c("BE", "BEL", "Bélgica", "Belgium", "Europa Occidental", "low", none, ""),
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
	c("HU", "HUN", "Hungría", "Hungary", "Europa Central", "low", none, ""),
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
	c("MC", "MCO", "Mónaco", "Monaco", "Europa Occidental", "low", none, ""),
	c("SM", "SMR", "San Marino", "San Marino", "Europa del Sur", "low", none, ""),
	c("LI", "LIE", "Liechtenstein", "Liechtenstein", "Europa Occidental", "low", none, ""),
	c("VA", "VAT", "Ciudad del Vaticano", "Vatican City", "Europa del Sur", "low", none, ""),
	c("XK", "XKX", "Kosovo", "Kosovo", "Europa", "low", none, ""),

	// Asia y Oceania
	c("CN", "CHN", "China", "China", "Asia Oriental", "low", none, ""),
	c("JP", "JPN", "Japón", "Japan", "Asia Oriental", "low", none, ""),
	c("KR", "KOR", "Corea del Sur", "South Korea", "Asia Oriental", "low", none, ""),
	c("TW", "TWN", "Taiwán", "Taiwan", "Asia Oriental", "low", none, ""),
	c("HK", "HKG", "Hong Kong", "Hong Kong", "Asia Oriental", "low", none, ""),
	c("MO", "MAC", "Macao", "Macao", "Asia Oriental", "low", none, ""),
	c("SG", "SGP", "Singapur", "Singapore", "Sudeste Asiático", "low", none, ""),
	c("MY", "MYS", "Malasia", "Malaysia", "Sudeste Asiático", "low", none, ""),
	c("TH", "THA", "Tailandia", "Thailand", "Sudeste Asiático", "low", none, ""),
	c("ID", "IDN", "Indonesia", "Indonesia", "Sudeste Asiático", "low", none, ""),
	c("BN", "BRN", "Brunei", "Brunei", "Sudeste Asiático", "low", none, ""),
	c("TL", "TLS", "Timor Oriental", "Timor-Leste", "Sudeste Asiático", "low", none, ""),
	c("IN", "IND", "India", "India", "Asia del Sur", "low", none, ""),
	c("BT", "BTN", "Bután", "Bhutan", "Asia del Sur", "low", none, ""),
	c("MV", "MDV", "Maldivas", "Maldives", "Asia del Sur", "low", none, ""),
	c("AU", "AUS", "Australia", "Australia", "Oceanía", "low", none, ""),
	c("NZ", "NZL", "Nueva Zelanda", "New Zealand", "Oceanía", "low", none, ""),
	c("FJ", "FJI", "Fiyi", "Fiji", "Oceanía", "low", none, ""),
	c("PG", "PNG", "Papua Nueva Guinea", "Papua New Guinea", "Oceanía", "low", none, ""),
	c("WS", "WSM", "Samoa", "Samoa", "Oceanía", "low", none, ""),
	c("TO", "TON", "Tonga", "Tonga", "Oceanía", "low", none, ""),
	c("VU", "VUT", "Vanuatu", "Vanuatu", "Oceanía", "low", none, ""),
	c("KZ", "KAZ", "Kazajistán", "Kazakhstan", "Asia Central", "low", none, ""),

	// Medio Oriente
	c("AE", "ARE", "Emiratos Árabes Unidos", "United Arab Emirates", "Medio Oriente", "low", none, ""),
	c("SA", "SAU", "Arabia Saudita", "Saudi Arabia", "Medio Oriente", "low", none, ""),
	c("QA", "QAT", "Catar", "Qatar", "Medio Oriente", "low", none, ""),
	c("KW", "KWT", "Kuwait", "Kuwait", "Medio Oriente", "low", none, ""),
	c("BH", "BHR", "Baréin", "Bahrain", "Medio Oriente", "low", none, ""),
	c("OM", "OMN", "Omán", "Oman", "Medio Oriente", "low", none, ""),
	c("JO", "JOR", "Jordania", "Jordan", "Medio Oriente", "low", none, ""),
	c("IL", "ISR", "Israel", "Israel", "Medio Oriente", "low", none, ""),
	c("PS", "PSE", "Palestina", "Palestine", "Medio Oriente", "low", none, ""),

	// Africa (low risk)
	c("MA", "MAR", "Marruecos", "Morocco", "África del Norte", "low", none, ""),
	c("TN", "TUN", "Túnez", "Tunisia", "África del Norte", "low", none, ""),
	c("DZ", "DZA", "Argelia", "Algeria", "África del Norte", "low", none, ""),
	c("EG", "EGY", "Egipto", "Egypt", "África del Norte", "low", none, ""),
	c("MU", "MUS", "Mauricio", "Mauritius", "África Oriental", "low", none, ""),
	c("BW", "BWA", "Botsuana", "Botswana", "África Austral", "low", none, ""),
	c("NA", "NAM", "Namibia", "Namibia", "África Austral", "low", none, ""),
	c("SC", "SYC", "Seychelles", "Seychelles", "África Oriental", "low", none, ""),
	c("CV", "CPV", "Cabo Verde", "Cape Verde", "África Occidental", "low", none, ""),
	c("CI", "CIV", "Costa de Marfil", "Ivory Coast", "África Occidental", "low", none, ""),
	c("TG", "TGO", "Togo", "Togo", "África Occidental", "low", none, ""),
	c("BJ", "BEN", "Benin", "Benin", "África Occidental", "low", none, ""),
	c("GM", "GMB", "Gambia", "Gambia", "África Occidental", "low", none, ""),
	c("GW", "GNB", "Guinea-Bisáu", "Guinea-Bissau", "África Occidental", "low", none, ""),
	c("GA", "GAB", "Gabón", "Gabon", "África Central", "low", none, ""),
	c("CG", "COG", "Rep. del Congo", "Republic of Congo", "África Central", "low", none, ""),
	c("GQ", "GNQ", "Guinea Ecuatorial", "Equatorial Guinea", "África Central", "low", none, ""),
	c("AO", "AGO", "Angola", "Angola", "África Austral", "low", none, ""),
	c("ZM", "ZMB", "Zambia", "Zambia", "África Austral", "low", none, ""),
	c("MW", "MWI", "Malaui", "Malawi", "África Oriental", "low", none, ""),
	c("LS", "LSO", "Lesoto", "Lesotho", "África Austral", "low", none, ""),
	c("SZ", "SWZ", "Esuatini", "Eswatini", "África Austral", "low", none, ""),
	c("KM", "COM", "Comoras", "Comoros", "África Oriental", "low", none, ""),
	c("ST", "STP", "Santo Tomé y Príncipe", "Sao Tome and Principe", "África Central", "low", none, ""),
}
