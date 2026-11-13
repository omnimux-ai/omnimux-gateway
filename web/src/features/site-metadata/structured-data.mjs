/** Preserve JSON values without allowing a value to close the HTML script. */
export function serializeStructuredData(graph) {
  return JSON.stringify(graph)
    .replaceAll('<', '\\u003c')
    .replaceAll('>', '\\u003e')
    .replaceAll('&', '\\u0026')
}
