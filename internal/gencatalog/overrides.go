package gencatalog

// titleOverrides is gen-catalog.py TITLE_OVERRIDES: curated catalog titles that win over
// the block H1 (a descriptive gloss for H1s without a title, or a Spanish gloss of an
// English H1). Copied verbatim from the Python source.
var titleOverrides = map[string]string{
	"27":  "Network surface + puertos + certManagement",
	"28":  "Discovery cross-protocol + Virtual Components",
	"29":  "Web tier + Servlets + Jetty filter chain",
	"30":  "Enterprise auth federation + FIPS + key rotation",
	"31":  "Performance tuning + observability + thread pools",
	"32":  "Honeywell modules + SMA + non-HTTP + runtime semantics",
	"33":  "History system deep + Batch editor",
	"34":  "Alarm framework deep + .adb format",
	"35":  "Nav + Workbench shell + views registry",
	"45":  "History/Trend chart consumption",
	"271": "BACnet discovery→punto: `objectTypes.xml` como tabla maestra, decisión Numeric/Boolean/Enum vía ASN type, read-only vs writable, derivación de facets — corrige 3 errores de B28",
	"272": "COV deep-dive bidireccional: gate cliente, máquina de 10 estados, **resubscribe = lifetime ÷ 2**, y el motor servidor — corrige 3 números de B23 §23.10",
	"273": "COV lado servidor: admisión de SubscribeCOV, **cap de 8 h hardcodeado**, dedup de 6 campos, suscripciones TRANSIENT (mueren en el restart), COV-Property es POLLED",
	"274": "Cierre de gaps COV: **defecto confirmado** — COV-Property con array index duplica y no se puede cancelar; poller de 5 s; `updateStatusOnCov` default false; Enum VALIDA donde Numeric coacciona",
	"275": "Familia export (I): 5 raíces, contrato `BIBacnetExportObject`, dispatch read/write por override, `Property_List` dinámico, `BacnetWritableDescriptor` = marker vacío",
	"276": "Familia export (II): **prioridad BACnet N ↔ slot inN vía BLink** (no es un write, es un cable); read total / write selectivo = control de acceso por nivel; `BacnetDescriptorUtil` = fábrica de puntos con 3er algoritmo de tipos divergente",
	"277": "Familia export (III): los 4 descriptors no-punto y el patrón «un objeto Niagara = una lista»; **los Schedule NO usan links de Niagara** — escriben por WriteProperty",
	"278": "Familia export (IV): **dos caminos distintos para exponer historia** (TrendLog vs NiagaraHistory), matriz de range types, delta doc↔código en el javadoc de Tridium, whitelist de 11 algoritmos de evento",
	"279": "**P3-mstp: hallazgo negativo** — el framing MS/TP no está en Java (`0x55` ausente en 50.798 archivos); vive en nativo. EMSTP es un falso rastro documentado. Reclasificado a requires-native-RE",
	"280": "**P3-sc cerrado**: BACnet/SC es 100% Java (40 clases, cero JNI), 13 function codes, header BVLC-SC, VMAC de 48 bits — corrige 2 números de B23 (49152 y TLS 1.3 no están en el código)",
	"281": "Encoding de Schedule: la cadena `ScheduleSupport16→4→0` elegida por **protocol revision del peer**, Weekly_Schedule como array de 7 con `% 7` puenteando días, 4 conversiones de fecha",
	"282": "Almacenamiento de trend records: `BBacnetTrendRecord extends BTrendRecord`; buffer-purge detectado por escritura con record **retrodatado 1 ms**; sequence que solo avanza sin fault",
	"283": "**TRES gates** en el Event Enrollment, distinguibles por código de error; matriz de `instanceof` sobre el punto Niagara; TODO de Tridium admite que las 4 variantes de out-of-range no se distinguen",
	"284": "Encoder ASN del trend record: los 3 range types colapsan en uno; sequence de 32 bits con wrap; maxSize por **encode-then-test**; layout de `BACnetLogRecord`",
	"285": "**EMSTP**, el protocolo host↔coprocesador de Tridium: prefijo de 2 bits + opcode de 6, 17 comandos que mapean 1:1 con el JNI; el coprocesador es un **Atmel SAM4S** con firmware propio",
	"286": "Header options de BACnet/SC: marker de 1 byte con 4 campos; **el must-understand solo se hace valer en destination options**; las desconocidas conservan sus bytes",
	"287": "Topología y seguridad de BACnet/SC: failover de 6 estados con failback automático; dos certificados; **una conexión SC entrante se autentica como un `BUser` de Niagara**",
	"288": "Cierre del barrido: verificada la suposición de B276 sobre los writables (difieren solo en el chequeo de dominio); `Elapsed_Active_Time` = totalizador con intervalo forzado a 1 s + punto derivado",
}
