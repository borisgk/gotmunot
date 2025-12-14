package handlers

import (
	"database/sql"
	"html/template"
)

var db *sql.DB
var tmpl *template.Template

func Init(database *sql.DB, t *template.Template) {
	db = database
	tmpl = t
}
