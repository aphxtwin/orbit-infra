internal/ Directory
Private application code for the Provisioning Controller

The internal/ directory contains all private packages of this microservice.
This is a Go language feature that enforces encapsulation at the module level.

Go treats any package inside an internal/ directory as unimportable from outside this repository.

That means:

only code inside this repo can import internal/...

external modules cannot import your internal logic

all sensitive or non-public implementation details stay private

This guarantees that your controller’s internal structure cannot leak or be accidentally depended on by other services.