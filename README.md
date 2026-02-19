# emp-go
TUI employee management system in Go

# Features
- Create employees, delete employees, clock-in, clock-out, multiple clock-ins and clock-outs, view all clock-ins and clock-outs

Data is kept in-memory only for now, so it becomes lost when you exit the app. I'll probably work on database support in the future!

# Installation
First, [install Go](https://go.dev/doc/install) if it's not installed already. Then run the following commands to build:
```
git clone https://github.com/AlanLuu/emp-go.git
cd emp-go
go build
```
This will create an executable binary called `emp` on Linux/macOS and `emp.exe` on Windows that can be run directly.

# Keyboard shortcuts
- up/down arrows: move selection in the employee list
- a: add a new employee
- d: delete the selected employee
- i: clock-in the selected employee
- o: clock-out the selected employee
- v: view all clock-ins and clock-outs of the selected employee
- q or Ctrl+C: quit

# License
MIT
