// Script to add missing MCCs to the catalog
// Run: docker exec -i datawatch-mongo mongosh datawatch < backend/scripts/add_mccs.js

const all = ["visa", "mastercard", "unionpay", "amex", "discover", "diners", "jcb"];

// Get existing codes to avoid duplicates
const existing = db.mccs.distinct("code");
const existingSet = new Set(existing);

const newMccs = [
  // ── Agricultura y Veterinaria ─────────────────────────────────
  {code: "0742", description: "Servicios veterinarios", category: "Agricultura", networks: all, risk_level: "low"},
  {code: "0763", description: "Cooperativas agricolas", category: "Agricultura", networks: all, risk_level: "low"},
  {code: "0780", description: "Servicios de jardineria y paisajismo", category: "Agricultura", networks: all, risk_level: "low"},

  // ── Construccion adicionales ───────────────────────────────────
  {code: "1521", description: "Contratistas generales - comercial", category: "Construccion", networks: all, risk_level: "low"},
  {code: "1542", description: "Contratistas generales - no residencial", category: "Construccion", networks: all, risk_level: "low"},

  // ── Transporte adicionales ────────────────────────────────────
  {code: "4011", description: "Ferrocarriles de carga", category: "Transporte", networks: all, risk_level: "low"},
  {code: "4119", description: "Servicios de ambulancia", category: "Transporte", networks: all, risk_level: "low"},
  {code: "4225", description: "Almacenamiento publico", category: "Transporte", networks: all, risk_level: "low"},
  {code: "4457", description: "Renta de botes", category: "Transporte", networks: all, risk_level: "low"},
  {code: "4511", description: "Lineas aereas (generico)", category: "Transporte", networks: all, risk_level: "medium"},
  {code: "4582", description: "Aeropuertos, terminales aereas", category: "Transporte", networks: all, risk_level: "low"},
  {code: "4733", description: "Servicios de paquetes turisticos", category: "Transporte", networks: all, risk_level: "medium"},

  // ── Aerolineas adicionales ────────────────────────────────────
  {code: "3019", description: "Air Lanka / SriLankan Airlines", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3022", description: "Air Inter", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3024", description: "Transportes Aereos Portugueses", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3025", description: "Olympic Airways", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3027", description: "UTA/Union de Transports Aeriens", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3028", description: "Air Malta", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3029", description: "SN Brussels Airlines", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3030", description: "Aerolineas Argentinas", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3037", description: "Air Pacific / Fiji Airways", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3040", description: "Ansett Airlines", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3041", description: "Finnair", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3042", description: "Austrian Airlines", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3043", description: "LOT Polish Airlines", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3046", description: "Cruzeiro do Sul", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3054", description: "Icelandair", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3056", description: "Lauda Air (Austrian)", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3057", description: "Tarom Romanian Airlines", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3059", description: "Air Zimbabwe", category: "Aerolineas", networks: all, risk_level: "high"},
  {code: "3061", description: "Air Austral", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3063", description: "Malev Hungarian Airlines", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3064", description: "Balkan Bulgarian Airlines", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3067", description: "Air Seychelles", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3070", description: "Malaysian Airlines", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3072", description: "Czech Airlines", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3073", description: "Kuwait Airways", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3074", description: "Gulf Air", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3075", description: "Singapore Airlines", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3076", description: "Egyptair", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3077", description: "Garuda Indonesia", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3078", description: "Royal Jordanian Airlines", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3079", description: "Pakistan International Airlines", category: "Aerolineas", networks: all, risk_level: "high"},
  {code: "3080", description: "Royal Brunei Airlines", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3081", description: "Oman Air", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3086", description: "Hainan Airlines", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3088", description: "Kenya Airways", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3089", description: "Uzbekistan Airways", category: "Aerolineas", networks: all, risk_level: "high"},
  {code: "3090", description: "Air Namibia", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3091", description: "Merpati Nusantara Airlines", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3092", description: "Air Vanuatu", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3093", description: "Air Tahiti Nui", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3094", description: "Air Caledonie", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3095", description: "Caribbean Airlines", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3097", description: "Alaska Airlines", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3098", description: "All Nippon Airways", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3100", description: "Cathay Pacific (alt)", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3102", description: "Air Mauritius", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3103", description: "Tunisair", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3106", description: "Air Europa", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3110", description: "Norwegian Air", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3111", description: "Virgin Atlantic", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3112", description: "Virgin Australia", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3115", description: "Air China", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3117", description: "Cebu Pacific Air", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3120", description: "WestJet Airlines", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3125", description: "EVA Air", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3127", description: "Asiana Airlines", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3130", description: "Flybe", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3131", description: "Brussels Airlines", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3132", description: "Sun Country Airlines", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3135", description: "Condor Flugdienst", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3137", description: "Pegasus Airlines", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3143", description: "Aer Lingus", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3144", description: "Vueling Airlines", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3145", description: "Air Baltic", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3148", description: "Wizz Air", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3156", description: "IndiGo Airlines", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3159", description: "SpiceJet", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3161", description: "AirAsia", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3170", description: "TAME (Ecuador)", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3171", description: "LAN Chile / LATAM Chile", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3172", description: "LAN Peru / LATAM Peru", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3173", description: "LAN Argentina / LATAM Argentina", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3175", description: "Aerolineas de Centroamerica", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3180", description: "Santa Barbara Airlines (Venezuela)", category: "Aerolineas", networks: all, risk_level: "high"},
  {code: "3185", description: "Conviasa (Venezuela)", category: "Aerolineas", networks: all, risk_level: "high"},
  {code: "3190", description: "Satena (Colombia)", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3195", description: "EcoJet (Bolivia)", category: "Aerolineas", networks: all, risk_level: "medium"},
  {code: "3200", description: "BOA Boliviana de Aviacion", category: "Aerolineas", networks: all, risk_level: "medium"},

  // ── Hoteles adicionales ───────────────────────────────────────
  {code: "3511", description: "Doubletree Hotels", category: "Hoteles", networks: all, risk_level: "low"},
  {code: "3513", description: "DoubleTree by Hilton", category: "Hoteles", networks: all, risk_level: "low"},
  {code: "3514", description: "Hampton Inn by Hilton", category: "Hoteles", networks: all, risk_level: "low"},
  {code: "3516", description: "Hilton Garden Inn", category: "Hoteles", networks: all, risk_level: "low"},
  {code: "3517", description: "Embassy Suites", category: "Hoteles", networks: all, risk_level: "low"},
  {code: "3518", description: "Homewood Suites", category: "Hoteles", networks: all, risk_level: "low"},
  {code: "3519", description: "Residence Inn by Marriott", category: "Hoteles", networks: all, risk_level: "low"},
  {code: "3520", description: "Courtyard by Marriott", category: "Hoteles", networks: all, risk_level: "low"},
  {code: "3521", description: "SpringHill Suites by Marriott", category: "Hoteles", networks: all, risk_level: "low"},
  {code: "3523", description: "La Quinta Inns", category: "Hoteles", networks: all, risk_level: "low"},
  {code: "3524", description: "Days Inn", category: "Hoteles", networks: all, risk_level: "low"},
  {code: "3525", description: "Super 8 Motels", category: "Hoteles", networks: all, risk_level: "low"},
  {code: "3526", description: "Ramada Hotels", category: "Hoteles", networks: all, risk_level: "low"},
  {code: "3528", description: "Howard Johnson Hotels", category: "Hoteles", networks: all, risk_level: "low"},
  {code: "3529", description: "Comfort Inn / Choice Hotels", category: "Hoteles", networks: all, risk_level: "low"},
  {code: "3530", description: "Econo Lodge", category: "Hoteles", networks: all, risk_level: "low"},
  {code: "3531", description: "Quality Inn", category: "Hoteles", networks: all, risk_level: "low"},
  {code: "3532", description: "Clarion Hotels", category: "Hoteles", networks: all, risk_level: "low"},
  {code: "3534", description: "W Hotels", category: "Hoteles", networks: all, risk_level: "low"},
  {code: "3535", description: "InterContinental Hotels", category: "Hoteles", networks: all, risk_level: "low"},
  {code: "3536", description: "Kimpton Hotels", category: "Hoteles", networks: all, risk_level: "low"},
  {code: "3537", description: "Loews Hotels", category: "Hoteles", networks: all, risk_level: "low"},
  {code: "3538", description: "Mandarin Oriental", category: "Hoteles", networks: all, risk_level: "low"},
  {code: "3539", description: "Movenpick Hotels", category: "Hoteles", networks: all, risk_level: "low"},
  {code: "3540", description: "Omni Hotels", category: "Hoteles", networks: all, risk_level: "low"},
  {code: "3541", description: "Park Hyatt Hotels", category: "Hoteles", networks: all, risk_level: "low"},
  {code: "3542", description: "Grand Hyatt Hotels", category: "Hoteles", networks: all, risk_level: "low"},
  {code: "3543", description: "Peninsula Hotels", category: "Hoteles", networks: all, risk_level: "low"},
  {code: "3544", description: "Rosewood Hotels", category: "Hoteles", networks: all, risk_level: "low"},
  {code: "3545", description: "St. Regis Hotels", category: "Hoteles", networks: all, risk_level: "low"},
  {code: "3546", description: "JW Marriott Hotels", category: "Hoteles", networks: all, risk_level: "low"},
  {code: "3547", description: "Aloft Hotels", category: "Hoteles", networks: all, risk_level: "low"},
  {code: "3548", description: "Element Hotels", category: "Hoteles", networks: all, risk_level: "low"},

  // ── Renta de Autos adicionales ────────────────────────────────
  {code: "3356", description: "Payless Car Rental", category: "Renta de Autos", networks: all, risk_level: "low"},
  {code: "3358", description: "Thrifty Car Rental", category: "Renta de Autos", networks: all, risk_level: "low"},
  {code: "3360", description: "Europcar", category: "Renta de Autos", networks: all, risk_level: "low"},
  {code: "3361", description: "Sixt Rent A Car", category: "Renta de Autos", networks: all, risk_level: "low"},
  {code: "3362", description: "Advantage Rent A Car", category: "Renta de Autos", networks: all, risk_level: "low"},
  {code: "3366", description: "Fox Rent A Car", category: "Renta de Autos", networks: all, risk_level: "low"},
  {code: "3368", description: "Zipcar", category: "Renta de Autos", networks: all, risk_level: "low"},
  {code: "3370", description: "Turo (renta P2P)", category: "Renta de Autos", networks: all, risk_level: "medium"},
  {code: "3381", description: "National Car Rental (alt)", category: "Renta de Autos", networks: all, risk_level: "low"},
  {code: "3385", description: "Rentas de autos miscelaneas", category: "Renta de Autos", networks: all, risk_level: "low"},
  {code: "3389", description: "Rentas de autos no clasificadas", category: "Renta de Autos", networks: all, risk_level: "low"},
  {code: "3390", description: "Uber Rent", category: "Renta de Autos", networks: all, risk_level: "low"},
  {code: "3393", description: "National Car Rental (Intl)", category: "Renta de Autos", networks: all, risk_level: "low"},
  {code: "3395", description: "Booking.com Rent Car", category: "Renta de Autos", networks: all, risk_level: "low"},
  {code: "3398", description: "Localiza Rent a Car", category: "Renta de Autos", networks: all, risk_level: "low"},
  {code: "3400", description: "Auto Europe", category: "Renta de Autos", networks: all, risk_level: "low"},
  {code: "3405", description: "Rentas de autos LATAM", category: "Renta de Autos", networks: all, risk_level: "low"},
  {code: "3409", description: "General Rent-A-Car", category: "Renta de Autos", networks: all, risk_level: "low"},
  {code: "3412", description: "A-1 Rent-A-Car", category: "Renta de Autos", networks: all, risk_level: "low"},
  {code: "3420", description: "Dollar Rent-A-Car (intl)", category: "Renta de Autos", networks: all, risk_level: "low"},
  {code: "3421", description: "Snappy Rent-A-Car", category: "Renta de Autos", networks: all, risk_level: "low"},
  {code: "3425", description: "Autonom Rent A Car", category: "Renta de Autos", networks: all, risk_level: "low"},
  {code: "3429", description: "Rentas de autos boutique", category: "Renta de Autos", networks: all, risk_level: "low"},
  {code: "3430", description: "Ace Rent A Car", category: "Renta de Autos", networks: all, risk_level: "low"},
  {code: "3431", description: "U-Save Auto Rental", category: "Renta de Autos", networks: all, risk_level: "low"},
  {code: "3432", description: "Routes Car Rental", category: "Renta de Autos", networks: all, risk_level: "low"},
  {code: "3433", description: "Green Motion Car Rental", category: "Renta de Autos", networks: all, risk_level: "low"},
  {code: "3434", description: "Goldcar Rental", category: "Renta de Autos", networks: all, risk_level: "low"},
  {code: "3435", description: "Rent-A-Wreck", category: "Renta de Autos", networks: all, risk_level: "low"},
  {code: "3436", description: "Priceless Car Rental", category: "Renta de Autos", networks: all, risk_level: "low"},
  {code: "3438", description: "Klass Wagen (Panama)", category: "Renta de Autos", networks: all, risk_level: "low"},
  {code: "3439", description: "Mex Rent A Car", category: "Renta de Autos", networks: all, risk_level: "low"},

  // ── Retail adicional ──────────────────────────────────────────
  {code: "5013", description: "Autopartes al por mayor", category: "Retail Automotriz", networks: all, risk_level: "low"},
  {code: "5039", description: "Materiales de construccion NEC", category: "Retail Construccion", networks: all, risk_level: "low"},
  {code: "5044", description: "Equipo de oficina, fotocopia, micro", category: "Retail Oficina", networks: all, risk_level: "low"},
  {code: "5046", description: "Equipo comercial NEC", category: "Retail Comercial", networks: all, risk_level: "low"},
  {code: "5047", description: "Equipo medico y dental al por mayor", category: "Farmacia y Salud", networks: all, risk_level: "low"},
  {code: "5051", description: "Centros de servicio metalurgico", category: "Industrial", networks: all, risk_level: "low"},
  {code: "5072", description: "Ferreteria al por mayor", category: "Industrial", networks: all, risk_level: "low"},
  {code: "5074", description: "Plomeria y calefaccion al por mayor", category: "Industrial", networks: all, risk_level: "low"},
  {code: "5085", description: "Suministros industriales NEC", category: "Industrial", networks: all, risk_level: "low"},
  {code: "5099", description: "Bienes durables NEC", category: "Industrial", networks: all, risk_level: "low"},
  {code: "5111", description: "Papeleria y articulos de oficina al por mayor", category: "Retail Oficina", networks: all, risk_level: "low"},
  {code: "5139", description: "Calzado comercial al por mayor", category: "Ropa y Moda", networks: all, risk_level: "low"},
  {code: "5199", description: "Bienes no durables NEC", category: "Otros", networks: all, risk_level: "low"},

  // ── Servicios Financieros adicionales ─────────────────────────
  {code: "6012", description: "Productos financieros bancarios", category: "Servicios Financieros", networks: all, risk_level: "medium"},
  {code: "6051", description: "Cuasi-efectivo no financiero", category: "Servicios Financieros", networks: all, risk_level: "high"},
  {code: "6399", description: "Seguros - no clasificados", category: "Servicios Financieros", networks: all, risk_level: "medium"},
  {code: "6529", description: "Remesas / envio de dinero remoto", category: "Servicios Financieros", networks: all, risk_level: "high"},

  // ── Digital y Tecnologia adicionales ──────────────────────────
  {code: "4814", description: "Servicios de telecomunicaciones fax", category: "Telecomunicaciones", networks: all, risk_level: "low"},
  {code: "5815", description: "Libros digitales", category: "Tecnologia", networks: all, risk_level: "low"},
  {code: "5819", description: "Digitales (generico)", category: "Tecnologia", networks: all, risk_level: "low"},

  // ── Servicios Profesionales adicionales ────────────────────────
  {code: "7210", description: "Servicios de lavanderia", category: "Servicios Varios", networks: all, risk_level: "low"},
  {code: "7277", description: "Servicios de consejeria y deuda", category: "Servicios Profesionales", networks: all, risk_level: "medium"},
  {code: "7333", description: "Fotografia comercial e impresion", category: "Servicios Profesionales", networks: all, risk_level: "low"},
  {code: "7338", description: "Copias y reproduccion", category: "Servicios Profesionales", networks: all, risk_level: "low"},
  {code: "7361", description: "Agencias de empleo", category: "Servicios Profesionales", networks: all, risk_level: "low"},
  {code: "7372", description: "Software y programacion", category: "Servicios Profesionales", networks: all, risk_level: "low"},
  {code: "7375", description: "Servicios de informacion y datos", category: "Servicios Profesionales", networks: all, risk_level: "low"},
  {code: "7379", description: "Servicios de computacion NEC", category: "Servicios Profesionales", networks: all, risk_level: "low"},
  {code: "7393", description: "Agencias detectivescas y proteccion", category: "Servicios Profesionales", networks: all, risk_level: "medium"},
  {code: "7394", description: "Alquiler de equipo", category: "Servicios Profesionales", networks: all, risk_level: "low"},
  {code: "7395", description: "Laboratorios fotograficos", category: "Servicios Profesionales", networks: all, risk_level: "low"},
  {code: "8734", description: "Laboratorios de pruebas NEC", category: "Servicios Profesionales", networks: all, risk_level: "low"},
  {code: "8911", description: "Servicios de arquitectura e ingenieria", category: "Servicios Profesionales", networks: all, risk_level: "low"},
  {code: "8999", description: "Servicios profesionales NEC", category: "Servicios Profesionales", networks: all, risk_level: "medium"},

  // ── Gobierno adicionales ──────────────────────────────────────
  {code: "9211", description: "Cortes y multas judiciales", category: "Gobierno", networks: all, risk_level: "low"},
  {code: "9223", description: "Fianzas", category: "Gobierno", networks: all, risk_level: "medium"},
  {code: "9311", description: "Pagos de impuestos federales", category: "Gobierno", networks: all, risk_level: "low"},
  {code: "9399", description: "Servicios gubernamentales NEC", category: "Gobierno", networks: all, risk_level: "low"},
  {code: "9401", description: "Gobierno - servicios postales", category: "Gobierno", networks: all, risk_level: "low"},
  {code: "9402", description: "Servicio postal gubernamental", category: "Gobierno", networks: all, risk_level: "low"},
  {code: "9405", description: "Agencias gubernamentales intra-gobierno", category: "Gobierno", networks: all, risk_level: "low"},
  {code: "9700", description: "Pagos automatizados del gobierno", category: "Gobierno", networks: all, risk_level: "low"},
  {code: "9701", description: "Pagos VISA al gobierno", category: "Gobierno", networks: all, risk_level: "low"},
  {code: "9702", description: "Pagos MC al gobierno", category: "Gobierno", networks: all, risk_level: "low"},
  {code: "9751", description: "Pasajes UK", category: "Gobierno", networks: all, risk_level: "low"},
  {code: "9752", description: "Impuestos UK", category: "Gobierno", networks: all, risk_level: "low"},
  {code: "9754", description: "Pagos de multas de transito", category: "Gobierno", networks: all, risk_level: "low"},

  // ── Armas y Defensa ───────────────────────────────────────────
  {code: "5571", description: "Distribuidores de motocicletas y scooters", category: "Vehiculos", networks: all, risk_level: "low"},
  {code: "5592", description: "Casas rodantes", category: "Vehiculos", networks: all, risk_level: "low"},
  {code: "5598", description: "Distribuidores de motonieve", category: "Vehiculos", networks: all, risk_level: "low"},
  {code: "5599", description: "Distribuidores vehiculos miscelaneos", category: "Vehiculos", networks: all, risk_level: "low"},
  {code: "5940", description: "Tiendas de bicicletas", category: "Vehiculos", networks: all, risk_level: "low"},
  {code: "5551", description: "Distribuidores de botes y lanchas", category: "Vehiculos", networks: all, risk_level: "medium"},
  {code: "5561", description: "Distribuidores de casas rodantes", category: "Vehiculos", networks: all, risk_level: "low"},
  {code: "5571", description: "Distribuidores de motocicletas", category: "Vehiculos", networks: all, risk_level: "low"},

  // ── Armas y municiones (alto riesgo compliance) ───────────────
  {code: "5941", description: "Articulos deportivos (incluye armas deportivas)", category: "Armas y Municiones", networks: all, risk_level: "high"},

  // ── Criptomonedas y Fintech ───────────────────────────────────
  {code: "6051", description: "Casas de cambio y criptomonedas", category: "Criptomonedas y Fintech", networks: all, risk_level: "high"},
  {code: "6211", description: "Corredores de valores y criptoactivos", category: "Criptomonedas y Fintech", networks: all, risk_level: "high"},
  {code: "6540", description: "Tarjetas prepago / gift cards cripto", category: "Criptomonedas y Fintech", networks: all, risk_level: "high"},

  // ── Organizaciones adicionales ────────────────────────────────
  {code: "8641", description: "Asociaciones civicas y fraternales", category: "Organizaciones", networks: all, risk_level: "low"},
  {code: "8651", description: "Organizaciones politicas", category: "Organizaciones", networks: all, risk_level: "medium"},
  {code: "8661", description: "Organizaciones religiosas", category: "Organizaciones", networks: all, risk_level: "medium"},
  {code: "8675", description: "Asociaciones de automoviles", category: "Organizaciones", networks: all, risk_level: "low"},
  {code: "8699", description: "Organizaciones de membresia NEC", category: "Organizaciones", networks: all, risk_level: "medium"},

  // ── Educacion adicional ───────────────────────────────────────
  {code: "8211", description: "Escuelas primarias/secundarias", category: "Educacion", networks: all, risk_level: "low"},
  {code: "8220", description: "Universidades", category: "Educacion", networks: all, risk_level: "low"},
  {code: "8241", description: "Escuelas por correspondencia", category: "Educacion", networks: all, risk_level: "low"},
  {code: "8244", description: "Escuelas de negocios", category: "Educacion", networks: all, risk_level: "low"},
  {code: "8249", description: "Escuelas vocacionales", category: "Educacion", networks: all, risk_level: "low"},
  {code: "8299", description: "Educacion NEC", category: "Educacion", networks: all, risk_level: "low"},

  // ── Salud y Bienestar adicionales ─────────────────────────────
  {code: "5047", description: "Equipo medico al por mayor", category: "Farmacia y Salud", networks: all, risk_level: "low"},
  {code: "5975", description: "Aparatos para sordera", category: "Farmacia y Salud", networks: all, risk_level: "low"},
  {code: "5976", description: "Aparatos ortopedicos", category: "Farmacia y Salud", networks: all, risk_level: "low"},
  {code: "8011", description: "Medicos", category: "Farmacia y Salud", networks: all, risk_level: "low"},
  {code: "8021", description: "Dentistas", category: "Farmacia y Salud", networks: all, risk_level: "low"},
  {code: "8031", description: "Osteopatas", category: "Farmacia y Salud", networks: all, risk_level: "low"},
  {code: "8041", description: "Quiropracticos", category: "Farmacia y Salud", networks: all, risk_level: "low"},
  {code: "8042", description: "Optometras", category: "Farmacia y Salud", networks: all, risk_level: "low"},
  {code: "8043", description: "Oftalmologos", category: "Farmacia y Salud", networks: all, risk_level: "low"},
  {code: "8049", description: "Podiatras", category: "Farmacia y Salud", networks: all, risk_level: "low"},
  {code: "8050", description: "Cuidado de enfermeria", category: "Farmacia y Salud", networks: all, risk_level: "low"},
  {code: "8062", description: "Hospitales", category: "Farmacia y Salud", networks: all, risk_level: "low"},
  {code: "8071", description: "Laboratorios medicos", category: "Farmacia y Salud", networks: all, risk_level: "low"},
  {code: "8099", description: "Servicios medicos NEC", category: "Farmacia y Salud", networks: all, risk_level: "low"},

  // ── Inmobiliario ──────────────────────────────────────────────
  {code: "6513", description: "Agentes inmobiliarios y administradores", category: "Inmobiliario", networks: all, risk_level: "high"},
  {code: "1522", description: "Contratistas generales residenciales", category: "Inmobiliario", networks: all, risk_level: "low"},
  {code: "1542", description: "Contratistas generales no residenciales", category: "Inmobiliario", networks: all, risk_level: "low"},
  {code: "6513", description: "Bienes raices", category: "Inmobiliario", networks: all, risk_level: "high"},

  // ── Tabacos y Alcohol (compliance) ────────────────────────────
  {code: "5921", description: "Licorerías y vinos", category: "Tabacos y Alcohol", networks: all, risk_level: "medium"},
  {code: "5993", description: "Cigarros, tabacos y puros", category: "Tabacos y Alcohol", networks: all, risk_level: "medium"},
  {code: "5813", description: "Bares y cantinas", category: "Tabacos y Alcohol", networks: all, risk_level: "medium"},

  // ── Seguros ───────────────────────────────────────────────────
  {code: "6300", description: "Seguros - ventas y suscripciones", category: "Seguros", networks: all, risk_level: "medium"},
  {code: "6381", description: "Seguros - primas", category: "Seguros", networks: all, risk_level: "medium"},
  {code: "6399", description: "Seguros - no clasificados", category: "Seguros", networks: all, risk_level: "medium"},

  // ── Servicios Publicos adicionales ────────────────────────────
  {code: "4900", description: "Servicios publicos (electricidad, gas, agua)", category: "Servicios Publicos", networks: all, risk_level: "low"},
  {code: "4901", description: "Electricidad", category: "Servicios Publicos", networks: all, risk_level: "low"},
  {code: "4902", description: "Gas natural", category: "Servicios Publicos", networks: all, risk_level: "low"},
  {code: "4903", description: "Agua y alcantarillado", category: "Servicios Publicos", networks: all, risk_level: "low"},

  // ── NFT y Activos Digitales ───────────────────────────────────
  {code: "5815", description: "Contenido digital - libros electronicos", category: "Activos Digitales", networks: all, risk_level: "low"},
  {code: "5816", description: "Contenido digital - juegos", category: "Activos Digitales", networks: all, risk_level: "low"},
  {code: "5817", description: "Contenido digital - aplicaciones", category: "Activos Digitales", networks: all, risk_level: "low"},
  {code: "5818", description: "Marketplace digital", category: "Activos Digitales", networks: all, risk_level: "medium"},
];

// Filter out duplicates
const toInsert = newMccs.filter(m => !existingSet.has(m.code));

if (toInsert.length === 0) {
  print("No new MCCs to insert - all codes already exist.");
} else {
  const result = db.mccs.insertMany(toInsert, { ordered: false });
  print(`Inserted ${result.insertedCount} new MCCs. Total now: ${db.mccs.countDocuments()}`);
}
