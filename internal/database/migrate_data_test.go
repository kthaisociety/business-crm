package database_test

import (
	"context"
	"os"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/kthaisociety/business-crm/internal/database"
	"github.com/kthaisociety/business-crm/internal/database/databasetest"
)

// migrationsUpTo is the real migrations folder, cut off after the named file.
func migrationsUpTo(t *testing.T, names ...string) fstest.MapFS {
	fsys := fstest.MapFS{}
	for _, name := range names {
		data, err := os.ReadFile("migrations/" + name)
		require.NoError(t, err)
		fsys[name] = &fstest.MapFile{Data: data}
	}
	return fsys
}

// 00002 moves each deal's company name and contact fields into the new companies and contacts tables.
func TestCompaniesAndContactsMigrationMovesExistingDeals(t *testing.T) {
	ctx := context.Background()
	db := databasetest.OpenEmpty(t)
	require.NoError(t, database.MigrateFS(ctx, db, migrationsUpTo(t, "00001_create_deals.sql")))

	insert := func(company, name, email, role string) {
		require.NoError(t, db.Exec(`INSERT INTO deals (company, deal_type, semester, contact_name, contact_email, contact_role)
			VALUES (?, 'Event', 'HT 2026', ?, ?, ?)`, company, name, email, role).Error)
	}
	insert("Granit Consulting", "Lina Dahl", "lina.dahl@example.com", "Recruiter")
	insert(" granit consulting ", "Lina Dahl", "LINA.DAHL@example.com", "Recruitment Manager") // same company and person
	insert("Granit Consulting", "Per Ström", "", "Partner")                                    // no email
	insert("Granit Consulting", "per ström", "", "")                                           // same person, by name
	insert("Vinga Voice", "", "", "")                                                          // no contact
	insert("Vinga Voice", "Lina Dahl", "lina.dahl@example.com", "Advisor")                     // the same email at another company

	require.NoError(t, database.MigrateFS(ctx, db, migrationsUpTo(t, "00001_create_deals.sql", "00002_create_companies_and_contacts.sql")))

	var companies []string
	require.NoError(t, db.Raw(`SELECT name FROM companies ORDER BY name`).Scan(&companies).Error)
	assert.Equal(t, []string{"Granit Consulting", "Vinga Voice"}, companies)

	type contact struct{ Name, Email, Role, Company string }
	var contacts []contact
	require.NoError(t, db.Raw(`SELECT ct.name, ct.email, ct.role, c.name AS company
		FROM contacts ct JOIN companies c ON c.id = ct.company_id ORDER BY ct.name`).Scan(&contacts).Error)
	assert.ElementsMatch(t, []contact{
		{"Lina Dahl", "LINA.DAHL@example.com", "Recruitment Manager", "Granit Consulting"}, // the latest deal's details
		{"Lina Dahl", "lina.dahl@example.com", "Advisor", "Vinga Voice"},                   // a contact at each company
		{"per ström", "", "", "Granit Consulting"},
	}, contacts)

	var linked struct{ Deals, WithContact, Mismatched int }
	require.NoError(t, db.Raw(`SELECT count(*) AS deals, count(d.contact_id) AS with_contact,
		count(*) FILTER (WHERE ct.company_id <> d.company_id) AS mismatched
		FROM deals d LEFT JOIN contacts ct ON ct.id = d.contact_id`).Scan(&linked).Error)
	assert.Equal(t, 6, linked.Deals)
	assert.Equal(t, 5, linked.WithContact)
	assert.Zero(t, linked.Mismatched, "every deal's contact works at the deal's company")

	assertNoColumn(t, db, "deals", "contact_email")
}

func TestCompaniesAndContactsMigrationRollsBack(t *testing.T) {
	ctx := context.Background()
	db := databasetest.OpenEmpty(t)
	both := migrationsUpTo(t, "00001_create_deals.sql", "00002_create_companies_and_contacts.sql")
	require.NoError(t, database.MigrateFS(ctx, db, both))
	require.NoError(t, db.Exec(`INSERT INTO companies (id, name) VALUES ('11111111-1111-1111-1111-111111111111', 'Granit Consulting')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO contacts (id, company_id, name, email) VALUES
		('22222222-2222-2222-2222-222222222222', '11111111-1111-1111-1111-111111111111', 'Lina Dahl', 'lina.dahl@example.com')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO deals (company_id, contact_id, deal_type, semester) VALUES
		('11111111-1111-1111-1111-111111111111', '22222222-2222-2222-2222-222222222222', 'Event', 'HT 2026')`).Error)

	require.NoError(t, database.DownFS(ctx, db, both))

	var row struct{ Company, ContactEmail string }
	require.NoError(t, db.Raw(`SELECT company, contact_email FROM deals`).Scan(&row).Error)
	assert.Equal(t, "Granit Consulting", row.Company)
	assert.Equal(t, "lina.dahl@example.com", row.ContactEmail)
	assert.False(t, db.Migrator().HasTable("companies"))
}

func assertNoColumn(t *testing.T, db *gorm.DB, table, column string) {
	t.Helper()
	assert.False(t, db.Migrator().HasColumn(table, column), "%s.%s should be gone", table, column)
}
