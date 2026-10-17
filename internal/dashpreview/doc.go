// Package dashpreview ports tools/dashboard-preview.py: a local preview server
// for a Niagara N4 dashboard (-ux) module that serves its rc/ folder, mocks
// (or, with --live, proxies) the servlet's /api/*, enforces the XHR guard and
// can inject the label editor and the R14 config-login flow. Responses, server
// log lines and the startup banner match the Python tool.
package dashpreview
