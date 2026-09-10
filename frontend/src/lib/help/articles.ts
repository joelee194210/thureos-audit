import {
  LayoutDashboard, Bell, Upload, Monitor, ShieldCheck, ScanSearch,
  BarChart3, Bot, CreditCard, Globe, Users, ScrollText, Settings,
} from "lucide-react";
import type { HelpArticle } from "./types";

export const HELP_ARTICLES: HelpArticle[] = [
  {
    slug: "dashboard",
    title: "Dashboard",
    icon: LayoutDashboard,
    group: "Operación",
    route: "/",
    quienPuede:
      "Cualquier usuario con sesión iniciada. El panel es de solo lectura: no hay nada que crear ni editar acá.",
    summary:
      "Un pantallazo del estado del sistema: monitores, reglas activas, banderas rojas nuevas y dashboards, con lo último en banderas rojas y en monitores.",
    images: ["dashboard.png"],
    sections: [
      {
        heading: "¿Para qué sirve?",
        body:
          "Es la primera pantalla que ves al entrar. Junta en un solo lugar los números que importan para arrancar el día —cuántos monitores y reglas hay activos, cuántas banderas rojas llegaron sin revisar— junto con las últimas banderas rojas y los monitores con actividad reciente, para decidir a dónde ir después sin tener que visitar cada pantalla por separado.",
      },
      {
        heading: "¿Quién puede usarlo?",
        body:
          "Cualquier usuario con sesión iniciada ve el panel, con los mismos números para los tres roles. No hay ninguna acción para hacer acá: ni crear, ni editar, ni descartar.",
      },
      {
        heading: "Cómo usar el panel para arrancar el día",
        items: [
          "Mirá las cuatro tarjetas de arriba: monitores, reglas activas, banderas rojas nuevas y dashboards.",
          "Si «Banderas rojas nuevas» no está en cero, revisá la lista «Banderas rojas recientes» para ver qué reglas dispararon y con qué severidad.",
          "Hacé clic en el contador junto a la campana, arriba a la derecha, para ir a Banderas rojas e investigar cada una.",
          "Si un monitor de la lista «Monitores» tiene pocos registros o no cambió en varios días, entrá a Monitores para revisar si sigue recibiendo datos.",
        ],
      },
      {
        heading: "Por qué las listas no son clickeables",
        body:
          "Ni las banderas rojas ni los monitores que aparecen en el panel llevan a su detalle al hacer clic: son un resumen, no un atajo. Para investigar una bandera roja o revisar un monitor en profundidad, andá a la pantalla correspondiente desde el menú.",
      },
    ],
  },
  {
    slug: "banderas-rojas",
    title: "Banderas rojas",
    icon: Bell,
    group: "Operación",
    route: "/red-flags",
    quienPuede:
      "Ver las banderas rojas, cualquier usuario con sesión iniciada. Reconocerlas, resolverlas, descartarlas o dejar notas, los roles compliance y admin.",
    summary:
      "La cola central de alertas: cada coincidencia de una regla activa aparece acá para que alguien la reconozca, la resuelva o la descarte.",
    images: ["banderas-rojas.png"],
    sections: [
      {
        heading: "¿Para qué sirve?",
        body:
          "Acá aparece cada alerta que generó una regla al encontrar una coincidencia en los datos de un monitor. Es la cola de trabajo del analista: agrupa por severidad y estado para decidir qué mirar primero, con un calendario que muestra en qué días hubo actividad y estadísticas de nuevas, críticas, altas y agrupadas.",
      },
      {
        heading: "¿Quién puede usarlo?",
        body:
          "Cualquier usuario con sesión iniciada puede ver las banderas rojas y sus datos. Reconocerlas, resolverlas, descartarlas o dejar notas requiere el rol compliance o admin.",
      },
      {
        heading: "Cómo reconocer una bandera roja",
        items: [
          "Entrá a Banderas rojas. Usá el buscador o los filtros por monitor y estado para encontrar la que te interesa.",
          "En la tarjeta correspondiente, presioná «Reconocer».",
          "Elegí una categoría (por ejemplo, Investigación) y escribí en Notas qué acción tomaste — es obligatorio, no se puede confirmar en blanco.",
          "Confirmá. La bandera pasa a estado reconocido; desde ahí se puede Resolver o Descartar con el mismo tipo de diálogo.",
        ],
      },
      {
        heading: "Qué significa que una bandera esté Agregada",
        body:
          "Las banderas marcadas «Agregada» vienen de una condición que cuenta, suma o promedia sobre una ventana de tiempo —por ejemplo, más de 3 transacciones por tarjeta en 24 horas— y no de una sola fila. La tarjeta muestra el valor calculado contra el umbral de la regla y el porcentaje que representa, para entender qué tan por encima del límite quedó.",
      },
    ],
  },
  {
    slug: "cargas",
    title: "Cargas",
    icon: Upload,
    group: "Operación",
    route: "/uploads",
    quienPuede:
      "Cualquier usuario con sesión iniciada. Es una pantalla de solo lectura.",
    summary:
      "El historial de todas las cargas de datos, por día y por archivo, con el detalle de qué se aceptó y qué se rechazó en cada una.",
    images: ["cargas.png"],
    sections: [
      {
        heading: "¿Para qué sirve?",
        body:
          "Es el historial de todo lo que entró a la plataforma, cruzando todos los monitores: una tabla por día y monitor arriba, y una bitácora fila por fila de cada archivo subido abajo. No es donde se sube un archivo —eso se hace desde la pestaña «Cargar datos» del monitor— sino donde se audita lo que ya se subió.",
      },
      {
        heading: "¿Quién puede usarlo?",
        body:
          "Cualquier usuario con sesión iniciada. Es una pantalla de solo lectura: no hay ninguna acción de carga ni de edición acá.",
      },
      {
        heading: "Cómo encontrar una carga puntual",
        items: [
          "Entrá a Cargas.",
          "Filtrá por tipo de fuente, monitor, estado o rango de fechas, o buscá directamente por nombre de monitor.",
          "En la bitácora de abajo, ubicá el archivo por nombre y fecha/hora.",
          "Si la fila tiene un ícono para expandir, abrila para ver el detalle: las filas rechazadas con el motivo, o las columnas que no coincidían con el esquema.",
        ],
      },
      {
        heading: "Qué diferencia hay entre Aceptado, Parcial y Aprobado",
        body:
          "Aceptado es una carga que coincidió con el esquema del monitor sin problemas. Parcial es una carga donde algunas filas se rechazaron por no cumplir el tipo de un campo — el detalle dice cuál fila y por qué. Aprobado es una carga cuya estructura difería del esquema guardado —columnas de más o de menos— y que alguien revisó y dejó pasar igual; el detalle muestra qué columnas sobraban o faltaban.",
      },
    ],
  },
  {
    slug: "monitores",
    title: "Monitores",
    icon: Monitor,
    group: "Monitoreo",
    route: "/monitors",
    quienPuede:
      "Ver los monitores, cualquier usuario con sesión iniciada. Crearlos, editarlos y cargarles datos, los roles compliance y admin.",
    summary:
      "Cada monitor es una fuente de datos: define de dónde vienen las transacciones, qué columnas traen y cómo se interpretan.",
    images: ["monitores.png"],
    sections: [
      {
        heading: "¿Para qué sirve?",
        body:
          "Un monitor es el contenedor de una fuente de datos. Al cargar el primer archivo, el sistema detecta las columnas y arma el esquema: qué campos hay y de qué tipo es cada uno. A partir de ahí todo lo demás —las reglas, los tableros, el analista de IA— trabaja sobre ese esquema.",
      },
      {
        heading: "¿Quién puede usarlo?",
        body:
          "Cualquier usuario con sesión iniciada puede ver la lista y el detalle de un monitor. Crear uno nuevo, editar su esquema o cargarle datos requiere el rol compliance o admin. Un viewer ve los monitores pero no los modifica.",
      },
      {
        heading: "Cómo crear un monitor",
        items: [
          "Entrá a Monitores y presioná «Nuevo monitor».",
          "Ponele nombre y descripción, y elegí el tipo de fuente: CSV, TXT, Excel, JSON o API.",
          "Si es un archivo delimitado, indicá el separador (coma, punto y coma, pipe o tabulador) y si la primera fila es el encabezado.",
          "Guardá, entrá al monitor y cargá el primer archivo. El esquema se detecta solo.",
        ],
      },
      {
        heading: "Las cuatro pestañas de un monitor",
        items: [
          "Cargar datos: subís archivos. Antes de ingerir, el sistema compara la estructura con el esquema guardado y avisa si no coinciden.",
          "Datos: los registros ya ingeridos, con búsqueda.",
          "Esquema: el tipo de cada campo, los decimales implícitos y el timestamp derivado. Es la pestaña que más conviene entender — mirá la ficha «El esquema de un monitor».",
          "Reglas: las reglas que evalúan este monitor, con acceso directo a ejecutarlas.",
        ],
      },
      {
        heading: "Por qué una carga puede ser rechazada",
        body:
          "Si el archivo trae columnas distintas de las del esquema guardado, la carga se rechaza entera y se informa qué campos faltan o sobran. Es deliberado: aceptar un archivo con la estructura cambiada es la forma más silenciosa de arruinar un histórico. Si el cambio es legítimo, primero ajustá el esquema.",
      },
    ],
  },
  {
    slug: "reglas",
    title: "Reglas",
    icon: ShieldCheck,
    group: "Monitoreo",
    route: "/rules",
    quienPuede:
      "Ver las reglas, cualquier usuario con sesión iniciada. Crearlas, editarlas y ejecutarlas, los roles compliance y admin.",
    summary:
      "Las condiciones que se evalúan sobre los datos de un monitor. Cuando una se cumple, se genera una bandera roja.",
    images: ["reglas.png"],
    sections: [
      {
        heading: "¿Para qué sirve?",
        body:
          "Una regla describe un patrón que hay que detectar. Se evalúa automáticamente cada vez que entran datos nuevos, y también a pedido con «Ejecutar ahora» o de forma programada. Cada coincidencia genera una bandera roja que alguien tiene que investigar.",
      },
      {
        heading: "¿Quién puede usarlo?",
        body:
          "Cualquier usuario con sesión iniciada puede ver las reglas y sus resultados. Crear, editar, activar, desactivar o ejecutar una regla requiere el rol compliance o admin.",
      },
      {
        heading: "Los tres tipos de condición",
        items: [
          "Condiciones simples: se evalúan fila por fila. «El importe es mayor a 5000», «el MCC es 7995».",
          "Condiciones agregadas: agrupan y suman, cuentan o promedian sobre una ventana de tiempo anclada a ahora. «Más de 3 transacciones por tarjeta en 24 horas».",
          "Condiciones de velocidad: miden el tiempo entre una transacción y la anterior de la misma entidad. «Dos cargos de la misma tarjeta separados por menos de 35 segundos». Mirá la ficha «Condiciones de velocidad».",
        ],
      },
      {
        heading: "Cómo crear una regla",
        items: [
          "Entrá a Reglas y presioná «Nueva regla».",
          "Elegí el monitor sobre el que se evalúa y ponele un nombre que describa el patrón, no la implementación.",
          "Agregá las condiciones que necesites: simples, agregadas, de velocidad, o una combinación.",
          "Elegí la severidad y las acciones. Antes de guardar, «Probar contra histórico» te dice cuántas coincidencias habría producido sobre los datos ya cargados.",
        ],
      },
      {
        heading: "Por qué probar contra el histórico",
        body:
          "Una regla demasiado amplia genera cientos de banderas rojas que nadie alcanza a revisar, y el efecto práctico es que se dejan de mirar todas, incluidas las buenas. El botón «Probar contra histórico» corre la regla sobre los datos ya ingeridos sin guardar nada, para que puedas juzgar señal contra ruido antes de activarla.",
      },
      {
        heading: "Si una regla no dispara",
        body:
          "Revisá que el campo de tiempo de un agregado o de una condición de velocidad sea de tipo fecha: si es un número, la comparación no encuentra nada y la regla no avisa de eso. Al guardar, el sistema rechaza las condiciones que no podrían dispararse nunca y explica el motivo.",
      },
    ],
  },
  {
    slug: "screening",
    title: "Screening",
    icon: ScanSearch,
    group: "Monitoreo",
    route: "/screening",
    quienPuede:
      "Buscar acá requiere el rol compliance o admin. Un viewer no puede hacer búsquedas.",
    summary:
      "Búsqueda manual de un nombre contra las listas de sanciones OFAC, ONU, UE y UK, para consultas puntuales que no están atadas a ningún caso.",
    images: ["screening.png"],
    sections: [
      {
        heading: "¿Para qué sirve?",
        body:
          "Permite buscar un nombre, con fecha de nacimiento opcional, contra las listas de sanciones internacionales sin necesidad de que haya una transacción o una bandera roja de por medio. Sirve para resolver dudas puntuales —por ejemplo antes de aprobar una alta manual— cuando no hace falta dejar un registro formal.",
      },
      {
        heading: "¿Quién puede usarlo?",
        body:
          "Buscar acá requiere el rol compliance o admin. No es la herramienta para el screening que queda asociado a un caso: ese se ve desde el detalle de la bandera roja correspondiente.",
      },
      {
        heading: "Cómo hacer una búsqueda",
        items: [
          "Entrá a Screening.",
          "Escribí el nombre completo a buscar; la fecha de nacimiento es opcional pero ayuda a descartar coincidencias por homonimia.",
          "Presioná Buscar.",
          "Revisá el resultado: el estado (Limpio, Coincidencia, Revisar, Desestimado o Falso positivo) y, si hay coincidencias, la lista de origen y el puntaje de cada una.",
        ],
      },
      {
        heading: "Esta búsqueda no queda registrada",
        body:
          "A diferencia del screening que corre automáticamente sobre los datos de un monitor, una búsqueda hecha acá no se guarda en ningún caso ni queda en la bitácora de una bandera roja. Si necesitás dejar constancia de la revisión, hacela desde el caso correspondiente en Banderas rojas.",
      },
    ],
  },
  {
    slug: "dashboards",
    title: "Dashboards",
    icon: BarChart3,
    group: "Monitoreo",
    route: "/dashboards",
    quienPuede:
      "Ver los dashboards, cualquier usuario con sesión iniciada. Crearlos y agregarles o quitarles widgets, los roles compliance y admin.",
    summary:
      "Tableros personalizados con widgets que grafican los datos de un monitor o los resultados de una regla.",
    images: ["dashboards.png"],
    sections: [
      {
        heading: "¿Para qué sirve?",
        body:
          "Un dashboard agrupa widgets —gráficos de barras, líneas, torta, área o un simple número— que resumen datos de un monitor o el resultado de una regla, para tener de un vistazo lo que de otra forma requeriría revisar tabla por tabla.",
      },
      {
        heading: "¿Quién puede usarlo?",
        body:
          "Cualquier usuario con sesión iniciada puede ver los dashboards existentes y sus widgets. Crear un dashboard nuevo o agregarle o quitarle widgets requiere el rol compliance o admin.",
      },
      {
        heading: "Cómo agregar un widget",
        items: [
          "Entrá al dashboard y presioná «Agregar widget».",
          "Elegí el origen: Manual (armás el gráfico vos: monitor, campo, tipo de agregación y agrupación opcional) o Desde regla (el widget muestra cuántas coincidencias tuvo una regla existente).",
          "Completá el título y el tipo de gráfico.",
          "Presioná «Agregar widget».",
        ],
      },
      {
        heading: "Por qué un widget manual pide Campo y Agregación",
        body:
          "Un widget manual no muestra los datos crudos: los resume. «Agregación» define cómo —contar filas, sumar, promediar, mínimo o máximo— y «Campo» sobre qué columna. Si el campo elegido no es numérico, elegí «Contar»: sumar o promediar un campo de texto no da un resultado útil.",
      },
    ],
  },
  {
    slug: "analista-ia",
    title: "Analista IA",
    icon: Bot,
    group: "Monitoreo",
    route: "/chatbot",
    quienPuede:
      "Cualquier usuario con sesión iniciada. Es de solo lectura: consulta los datos que el monitor ya tiene, no los modifica.",
    summary:
      "Chat en lenguaje natural para hacer preguntas sobre los datos de un monitor puntual, con gráficos generados según la pregunta.",
    images: ["analista-ia.png"],
    sections: [
      {
        heading: "¿Para qué sirve?",
        body:
          "Permite preguntar en español, en lenguaje natural, sobre los datos ya cargados en un monitor —por ejemplo cuántas transacciones hay de un tipo, o cuál es el promedio de un campo— sin tener que armar una consulta ni un dashboard. Cada respuesta puede venir acompañada de un gráfico si la pregunta se presta.",
      },
      {
        heading: "¿Quién puede usarlo?",
        body:
          "Cualquier usuario con sesión iniciada. Es de solo lectura: el analista consulta los datos que el monitor ya tiene, no los modifica ni ejecuta reglas ni acciones.",
      },
      {
        heading: "Cómo hacer una consulta",
        items: [
          "Entrá a Analista IA y elegí un monitor en el desplegable de arriba.",
          "Escribí la pregunta en el cuadro de texto, en español simple.",
          "Enviala. La respuesta aparece en el panel central y, si corresponde, un gráfico en el panel de la derecha, descargable.",
          "Para arrancar de cero, presioná «Nueva conversación» — las conversaciones anteriores de ese monitor quedan listadas a la izquierda.",
        ],
      },
      {
        heading: "Las conversaciones son por monitor",
        body:
          "Al cambiar el monitor seleccionado, la lista de conversaciones de la izquierda cambia con él: una conversación no se comparte entre monitores. Si no encontrás una conversación anterior, revisá que esté seleccionado el mismo monitor con el que la hiciste.",
      },
    ],
  },
  {
    slug: "mcc",
    title: "MCC",
    icon: CreditCard,
    group: "Catálogos",
    route: "/mcc",
    quienPuede:
      "Consultar el catálogo, cualquier usuario con sesión iniciada. Editar el riesgo, la descripción o las redes de un MCC, los roles compliance y admin.",
    summary:
      "El catálogo de códigos de categoría de comercio (MCC), con el nivel de riesgo y las redes de tarjeta asociadas a cada uno.",
    images: ["mcc.png"],
    sections: [
      {
        heading: "¿Para qué sirve?",
        body:
          "Es el catálogo de referencia de los códigos MCC (Merchant Category Code) que identifican el rubro de cada comercio. Las reglas que evalúan el campo MCC de una transacción usan el nivel de riesgo asignado acá —Alto, Medio o Bajo— así que este catálogo es lo que define qué tan sensible es un rubro para el sistema.",
      },
      {
        heading: "¿Quién puede usarlo?",
        body:
          "Cualquier usuario con sesión iniciada puede consultar el catálogo, filtrar por categoría o nivel de riesgo y buscar por código o descripción. Editar el riesgo, la descripción o las redes de un MCC requiere el rol compliance o admin.",
      },
      {
        heading: "Cómo cambiar el riesgo de un MCC",
        items: [
          "Buscá el código o filtrá por categoría para encontrar el MCC.",
          "Presioná el ícono de lápiz en su fila.",
          "Cambiá el nivel de riesgo, la descripción o las redes habilitadas (Visa, Mastercard, UnionPay, American Express, Discover, Diners Club, JCB).",
          "Guardá los cambios.",
        ],
      },
      {
        heading: "Qué significa «Redes: Todas»",
        body:
          "Si un MCC tiene las siete redes habilitadas, la tabla lo resume como «Todas» en vez de listarlas una por una. Si necesitás que el riesgo asignado a un MCC solo aplique a transacciones de una red de tarjeta en particular, editalo y dejá tildadas únicamente esas redes.",
      },
    ],
  },
  {
    slug: "paises",
    title: "Países",
    icon: Globe,
    group: "Catálogos",
    route: "/countries",
    quienPuede:
      "Consultar el catálogo, cualquier usuario con sesión iniciada. Editar el riesgo, las fuentes, las notas o el estado de un país, los roles compliance y admin.",
    summary:
      "El catálogo de países con su nivel de riesgo, las listas de sanciones u organismos que lo justifican, y si está activo para el sistema.",
    images: ["paises.png"],
    sections: [
      {
        heading: "¿Para qué sirve?",
        body:
          "Es el catálogo de riesgo por país: cada uno tiene un nivel (Alto, Medio, Bajo), las fuentes que justifican ese nivel (FATF, OFAC, ONU, Basel, UE, entre otras) y una nota con el motivo. Las reglas que evalúan el país de origen o destino de una transacción usan este nivel.",
      },
      {
        heading: "¿Quién puede usarlo?",
        body:
          "Cualquier usuario con sesión iniciada puede consultar el catálogo y filtrar por región o nivel de riesgo. Editar el riesgo, las fuentes, las notas o el estado de un país requiere el rol compliance o admin.",
      },
      {
        heading: "Cómo cambiar el riesgo de un país",
        items: [
          "Filtrá por región o buscá el país en la lista.",
          "Presioná el ícono de lápiz en su fila — la fila se vuelve editable.",
          "Ajustá el nivel de riesgo, las fuentes (separadas por coma, por ejemplo «FATF, OFAC») y las notas.",
          "Confirmá con el ícono de check, o cancelá con la X.",
        ],
      },
      {
        heading: "Qué significa que un país esté Inactivo",
        body:
          "Un país «Inactivo» no se borra del catálogo: queda ahí, atenuado en la lista, como forma de dejarlo de lado sin perder su configuración ni su historial. Se reactiva desde el mismo botón de edición, tildando de nuevo «Activo».",
      },
    ],
  },
  {
    slug: "usuarios",
    title: "Usuarios",
    icon: Users,
    group: "Administración",
    route: "/users",
    quienPuede:
      "Solo el rol admin. Toda esta sección de Administración está restringida a administradores; ni compliance ni viewer pueden entrar.",
    summary:
      "Alta, edición de rol y baja de las cuentas de usuario del sistema. Solo para administradores.",
    images: ["usuarios.png"],
    sections: [
      {
        heading: "¿Para qué sirve?",
        body:
          "Es donde se crean las cuentas de acceso al sistema, se les asigna un rol —admin, compliance o viewer— y se las desactiva o elimina cuando corresponde. Es la única pantalla desde la que se gestionan usuarios.",
      },
      {
        heading: "¿Quién puede usarlo?",
        body:
          "Solo el rol admin. Toda esta sección de Administración está restringida a administradores; ni compliance ni viewer pueden entrar.",
      },
      {
        heading: "Cómo crear un usuario",
        items: [
          "Entrá a Usuarios y presioná «Nuevo usuario».",
          "Completá nombre, correo electrónico y contraseña.",
          "Elegí el rol: Administrador, Cumplimiento o Visualizador.",
          "Presioná Crear.",
        ],
      },
      {
        heading: "La diferencia entre Desactivar y Eliminar",
        body:
          "Desactivar bloquea el acceso de forma temporal y se puede revertir en cualquier momento. Eliminar es una baja permanente: el usuario no vuelve a aparecer en la lista ni puede iniciar sesión, y no se puede deshacer desde la interfaz. Tampoco podés desactivarte ni eliminarte a vos mismo: esos botones no aparecen en tu propia fila.",
      },
    ],
  },
  {
    slug: "bitacora-de-acceso",
    title: "Bitácora de acceso",
    icon: ScrollText,
    group: "Administración",
    route: "/activity-logs",
    quienPuede:
      "Solo el rol admin. Toda esta sección de Administración está restringida a administradores; ni compliance ni viewer pueden entrar.",
    summary:
      "El registro de auditoría de toda la actividad relevante del sistema: inicios de sesión, cambios de reglas, cargas de datos, gestión de usuarios y más. Solo para administradores.",
    images: ["bitacora-de-acceso.png"],
    sections: [
      {
        heading: "¿Para qué sirve?",
        body:
          "Es el registro de auditoría del sistema: cada inicio de sesión, acción sobre una bandera roja, cambio de regla, carga de datos, gestión de usuario o actualización del catálogo MCC queda anotado acá con quién, cuándo y desde qué IP. Sirve para reconstruir qué pasó cuando hace falta explicarlo.",
      },
      {
        heading: "¿Quién puede usarlo?",
        body:
          "Solo el rol admin. Toda esta sección de Administración está restringida a administradores; ni compliance ni viewer pueden entrar.",
      },
      {
        heading: "Cómo encontrar la actividad de un usuario",
        items: [
          "Entrá a Bitácora de acceso.",
          "Filtrá por tipo de acción (por ejemplo, «Regla actualizada» o «Carga de datos») o por usuario.",
          "Revisá la lista: cada entrada muestra el usuario, la acción, una descripción, la fecha/hora y la IP de origen.",
        ],
      },
      {
        heading: "Por qué hay tantos «Inicio de sesión»",
        body:
          "El grueso de la bitácora suele ser accesos, porque cada inicio de sesión queda registrado. Si estás buscando un cambio puntual, filtrá por el tipo de acción específico en vez de recorrer la lista completa.",
      },
    ],
  },
  {
    slug: "configuracion",
    title: "Configuración",
    icon: Settings,
    group: "Administración",
    route: "/settings",
    quienPuede:
      "Solo el rol admin. Toda esta sección de Administración está restringida a administradores; ni compliance ni viewer pueden entrar.",
    summary:
      "Los ajustes del sistema: notificaciones de banderas rojas, el servicio de screening, el proveedor de IA y el estado de los servicios de base. Solo para administradores.",
    images: ["configuracion.png"],
    sections: [
      {
        heading: "¿Para qué sirve?",
        body:
          "Reúne la configuración que normalmente no cambia todos los días: cómo se notifica una bandera roja nueva (email o webhook), la URL del servicio de screening de sanciones, qué proveedor y modelo de IA usa el Analista IA, y el estado de los servicios de base —MongoDB, Redis, el programador de reglas. Estas últimas secciones son de solo lectura, para diagnóstico.",
      },
      {
        heading: "¿Quién puede usarlo?",
        body:
          "Solo el rol admin. Toda esta sección de Administración está restringida a administradores; ni compliance ni viewer pueden entrar.",
      },
      {
        heading: "Cómo configurar el email de notificaciones",
        items: [
          "Entrá a Configuración y elegí el proveedor: SMTP directo o API de Resend.",
          "Completá los datos correspondientes —host, puerto, usuario y contraseña para SMTP, o la key de Resend— y los destinatarios, separados por coma.",
          "Presioná «Probar» para confirmar que el envío funciona antes de darlo por bueno.",
          "Presioná «Guardar notificaciones».",
        ],
      },
      {
        heading: "El tema claro/oscuro no es un ajuste del sistema",
        body:
          "El interruptor de Apariencia, arriba de todo, solo cambia el tema en el navegador donde lo tocás — se guarda ahí, no para todos los usuarios. El resto de la pantalla sí son ajustes compartidos por todo el sistema.",
      },
    ],
  },
];

/** La ficha de un slug, o undefined si no existe. */
export function articuloPorSlug(slug: string): HelpArticle | undefined {
  return HELP_ARTICLES.find((a) => a.slug === slug);
}
