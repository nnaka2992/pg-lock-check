# VHS script for pg-lock-check demo
# Run with: vhs docs/demo.tape

Output docs/demo.gif

Set FontSize 14
Set Width 1200
Set Height 600
Set Theme "Dracula"

Type "# pg-lock-check - PostgreSQL Lock Analyzer"
Enter
Sleep 1s

Type "# Check a simple SELECT query"
Enter
Sleep 500ms
Type "./pg-lock-check 'SELECT * FROM users'"
Enter
Sleep 2s

Type "# Check UPDATE without WHERE clause - CRITICAL!"
Enter
Sleep 500ms
Type "./pg-lock-check 'UPDATE users SET active = false'"
Enter
Sleep 2s

Type "# Check UPDATE with WHERE clause - WARNING"
Enter
Sleep 500ms
Type "./pg-lock-check 'UPDATE users SET active = false WHERE id = 1'"
Enter
Sleep 2s

Type "# Check multiple statements"
Enter
Sleep 500ms
Type "./pg-lock-check 'TRUNCATE TABLE users; CREATE INDEX idx_email ON users(email);'"
Enter
Sleep 2s

Type "# Check CREATE INDEX CONCURRENTLY in transaction mode - ERROR"
Enter
Sleep 500ms
Type "./pg-lock-check 'CREATE INDEX CONCURRENTLY idx_email ON users(email)'"
Enter
Sleep 2s

Type "# Check same with --no-transaction flag - WARNING"
Enter
Sleep 500ms
Type "./pg-lock-check --no-transaction 'CREATE INDEX CONCURRENTLY idx_email ON users(email)'"
Enter
Sleep 2s

Type "# Check from file"
Enter
Sleep 500ms
Type "echo 'CREATE TABLE users (id INT PRIMARY KEY);' > example.sql"
Enter
Sleep 500ms
Type "./pg-lock-check -f example.sql"
Enter
Sleep 2s

Type "# Clean up"
Enter
Type "rm example.sql"
Enter
Sleep 1s

Type "# Done! 🚀"
Enter
Sleep 2s