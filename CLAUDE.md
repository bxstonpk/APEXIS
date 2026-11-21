# Backend API Development Specification

## 1. Purpose

This document defines the mandatory development standards for implementing Backend APIs.

All APIs MUST follow:

- Go
- Hexagonal Architecture
- Clean Code principles
- Modular Monolith architecture
- GORM for database access
- PostgreSQL/MySQL-compatible relational database design
- Explicit separation between Domain, Application, Infrastructure, and Presentation layers

The AI MUST follow this specification when creating, modifying, or reviewing API code.

---

## 2. Mandatory Architecture

Each module MUST follow this structure:

```text
internal/modules/{module}/
├── domain/
│   ├── entities/
│   ├── value_objects/
│   ├── services/
│   └── errors/
│
├── application/
│   ├── ports/
│   │   └── repositories/
│   ├── dto/
│   └── usecases/
│
├── infrastructure/
│   └── persistence/
│       └── gorm/
│           ├── models/
│           ├── repositories/
│           ├── mapper.go
│           └── transaction.go
│
└── presentation/
    └── http/
        ├── handlers/
        ├── requests/
        └── responses/
```

Dependency direction MUST be:
```text
Presentation
     ↓
Application
     ↓
Application Ports
     ↓
Infrastructure
     ↓
GORM
     ↓
Database
```
Dependencies MUST NOT flow in the opposite direction.

## 3. Layer Responsibilities

### 3.1 Presentation Layer
Responsible for:
- HTTP request handling
- Request parsing
- Request validation
- HTTP status codes
- HTTP response formatting
- Authentication middleware integration
Presentation MUST NOT:
- Access GORM directly
- Access the database directly
- Implement business rules
- Execute SQL
- Implement transactions

Example:
```go
func (h *UserHandler) GetUser(c *gin.Context) {
    id := c.Param("id")

    result, err := h.useCase.Execute(
        c.Request.Context(),
        id,
    )

    if err != nil {
        // map application error to HTTP response
        return
    }

    c.JSON(http.StatusOK, result)
}
```

## 4. Application Layer
The Application Layer is responsible for:
- Use Case execution
- Business workflow orchestration
- Calling repositories through interfaces
- Transaction boundaries
- Authorization orchestration
- Coordinating multiple domain operations
- Mapping application-level errors
The Application Layer MUST NOT:
- Import GORM
- Import database-specific packages
- Execute SQL
- Know database table structures
- Depend on HTTP framework implementation
Example:
```go
type CreateUserUseCase struct {
    users ports.UserRepository
    tx    ports.TransactionManager
}

func (uc *CreateUserUseCase) Execute(
    ctx context.Context,
    input CreateUserInput,
) error {

    return uc.tx.WithinTransaction(ctx, func(ctx context.Context) error {

        user, err := entities.NewUser(
            input.Email,
            input.Name,
        )
        if err != nil {
            return err
        }

        if err := uc.users.Create(ctx, user); err != nil {
            return err
        }

        return nil
    })
}
```

## 5. Domain Layer
The Domain Layer owns business rules and business invariants.

Domain MAY contain:
- Entities
- Value Objects
- Domain Services
- Domain Errors
- Business Policies
Domain MUST NOT depend on:
- GORM
- PostgreSQL
- MySQL
- Gin
- HTTP
- Infrastructure
- Application implementation details
Example:
```go
type User struct {
    ID    string
    Email Email
    Name  string
}

func (u *User) ChangeName(name string) error {
    if name == "" {
        return ErrInvalidName
    }

    u.Name = name

    return nil
}
```

Business rules MUST NOT be implemented only in HTTP validation.

## 6. Repository Port
Repository interfaces MUST be defined outside Infrastructure.

Example:
```go
type UserRepository interface {
    Create(
        ctx context.Context,
        user *entities.User,
    ) error

    GetByID(
        ctx context.Context,
        id string,
    ) (*entities.User, error)

    Update(
        ctx context.Context,
        user *entities.User,
    ) error

    Delete(
        ctx context.Context,
        id string,
    ) error
}
```

The interface represents what the Application requires.

It MUST NOT expose:
```go
*gorm.DB
```

or any database-specific type.

## 7. GORM Responsibility
GORM MUST exist only in the Infrastructure Layer.

GORM MUST NOT appear in:
```text
domain/
application/
presentation/
```
GORM is responsible for:
- Database connection
- SQL generation
- CRUD implementation
- Query construction
- Transaction execution
- Persistence mapping
Example:
```text
infrastructure/
└── persistence/
    └── gorm/
        ├── models/
        ├── repositories/
        ├── mapper.go
        └── transaction.go
```

## 8. GORM Models
GORM Models MUST be separated from Domain Entities.

Example:
```go
type UserModel struct {
    ID        string    `gorm:"primaryKey"`
    Email     string    `gorm:"uniqueIndex;not null"`
    Name      string    `gorm:"not null"`
    CreatedAt time.Time `gorm:"not null"`
    UpdatedAt time.Time `gorm:"not null"`
}

func (UserModel) TableName() string {
    return "users"
}
```

DO NOT use GORM Model as Domain Entity.

DO NOT expose GORM Model directly as API Response.

## 9. Mapper
All conversions between persistence models and domain/application models MUST be explicit.

Example:
```go
func ToDomain(model models.UserModel) *entities.User {
    return &entities.User{
        ID:    model.ID,
        Email: model.Email,
        Name:  model.Name,
    }
}

func ToModel(entity *entities.User) models.UserModel {
    return models.UserModel{
        ID:    entity.ID,
        Email: entity.Email.Value(),
        Name:  entity.Name,
    }
}
```

## 10. CRUD Responsibility
CRUD operations MUST be implemented by Repository implementations inside Infrastructure.

Example:
```text
application/ports/user_repository.go
        ↓
interface UserRepository
        ↓
infrastructure/persistence/gorm/repositories/user_repository.go
        ↓
GORM
        ↓
Database
```

The GORM Repository is responsible for:
- Create
- Read
- Update
- Delete
- Query
- JOIN
- Filtering
- Sorting
- Pagination
- Persistence mapping

## 11. GET API Rules
### 11.1 Query Once
GET APIs MUST retrieve related data efficiently.

When required data can be retrieved using JOINs, the Repository MUST retrieve the required data using a single optimized database query.

Example:
```text
users
  JOIN departments
  JOIN companies
  JOIN roles
```
DO NOT implement N+1 queries.

BAD:
```go
user := getUser(id)

department := getDepartment(user.DepartmentID)

company := getCompany(user.CompanyID)

roles := getRoles(user.ID)
```

This can result in:
```text
Query 1 → User
Query 2 → Department
Query 3 → Company
Query 4 → Roles
```
GOOD:
```text
ONE optimized query

users
LEFT JOIN departments
LEFT JOIN companies
LEFT JOIN roles
```

## 12. GET Query Requirements
Queries MUST:
- Select only required columns
- Use JOIN when appropriate
- Use proper indexes
- Perform filtering in the database
- Perform sorting in the database
- Perform pagination in the database
- Avoid N+1 queries
- Avoid unnecessary database round trips
- Avoid SELECT * unless explicitly justified
Example:
```go
err := r.db.
    WithContext(ctx).
    Table("users u").
    Select(`
        u.id,
        u.email,
        u.name,
        d.id AS department_id,
        d.name AS department_name,
        c.id AS company_id,
        c.name AS company_name
    `).
    Joins(`
        LEFT JOIN departments d
            ON d.id = u.department_id
    `).
    Joins(`
        LEFT JOIN companies c
            ON c.id = u.company_id
    `).
    Where("u.id = ?", id).
    Scan(&result).
    Error
```

## 13. Important Query Rule
"Single Query" means:
Prefer one optimized query when the required data can reasonably be retrieved together.

DO NOT create an extremely complicated query merely to satisfy the single-query rule.

Multiple queries MAY be used when technically justified, such as:
- Large one-to-many datasets
- Complex pagination
- Independent datasets
- Separate aggregation requirements
- Query plans where multiple queries are demonstrably more efficient
If multiple queries are necessary, the AI MUST explain why.

## 14. GET API Transaction Rule
Normal GET operations SHOULD NOT use database transactions unnecessarily.

Default:
```text
GET
 ↓
Repository
 ↓
SELECT
```
If multiple reads require a consistent database snapshot, a read transaction MAY be used.

Do NOT create transactions simply because the API is a GET.

## 15. CREATE API Rules
All CREATE operations MUST use a database transaction.

Example:
```text
POST /users

BEGIN TRANSACTION

Validate
    ↓
Create User
    ↓
Create Roles
    ↓
Create Related Data
    ↓
Create Audit
    ↓
COMMIT
```
If any operation fails:
```text
ROLLBACK
```
No partial data MUST remain.

## 16. UPDATE API Rules
All UPDATE operations MUST use a database transaction.

Example:
```text
PUT /users/{id}

BEGIN TRANSACTION

Load required data
    ↓
Validate business rules
    ↓
Update User
    ↓
Update Related Data
    ↓
Create Audit
    ↓
COMMIT
```
Any error MUST cause:
```text
ROLLBACK
```

## 17. DELETE API Rules
DELETE operations that modify database state MUST use a transaction.

For soft delete:
```text
BEGIN TRANSACTION

Validate deletion rules
    ↓
Check dependencies
    ↓
Soft Delete
    ↓
Create Audit
    ↓
COMMIT
```

If any step fails:
```text
ROLLBACK
```

## 18. Transaction Boundary
Transactions MUST be controlled at the Application Use Case / Business Operation boundary.

Repositories MUST NOT independently create transactions for normal use-case operations.

BAD:
```go
func (r *UserRepository) Create(...) error {
    return r.db.Transaction(func(tx *gorm.DB) error {
        // ...
    })
}
```

This can create separate transactions for separate repositories.

GOOD:
```text
UseCase
    ↓
BEGIN TRANSACTION
    ↓
UserRepository
    ↓
RoleRepository
    ↓
AuditRepository
    ↓
COMMIT
```
All repositories MUST use the same transaction context.

## 19. TransactionManager
Application MUST depend on an abstraction.

Example:
```go
type TransactionManager interface {
    WithinTransaction(
        ctx context.Context,
        fn func(ctx context.Context) error,
    ) error
}
```

Infrastructure implements this interface using GORM.

Example:
```go
func (tm *GormTransactionManager) WithinTransaction(
    ctx context.Context,
    fn func(ctx context.Context) error,
) error {

    return tm.db.
        WithContext(ctx).
        Transaction(func(tx *gorm.DB) error {

            txCtx := WithTx(ctx, tx)

            return fn(txCtx)
        })
}
```

## 20. Same Transaction Requirement
All repositories participating in the same business operation MUST use the same transaction.

Example:
```text
CreateOrganization
        │
        ▼
BEGIN TRANSACTION
        │
        ├── CompanyRepository.Create()
        │
        ├── BranchRepository.Create()
        │
        ├── DepartmentRepository.Create()
        │
        ├── UserRepository.Create()
        │
        └── AuditRepository.Create()
        │
        ▼
     COMMIT
```
If any operation fails:
```text
ROLLBACK EVERYTHING
```

## 21. External Services and Transactions
DO NOT keep a database transaction open while waiting for external services.

BAD:
```text
BEGIN DB TRANSACTION
    ↓
Call AI Agent
    ↓
Call MCP
    ↓
Call External API
    ↓
Save DB
    ↓
COMMIT
```
This creates long-running database transactions.

Instead:
```text
Execute external operation
        ↓
Persist result
        ↓
Short DB Transaction
        ↓
COMMIT
```
For long-running workflows, use appropriate workflow/state-management patterns.

## 22. Validation Rules
Validation MUST be placed in the correct layer.
Presentation Validation
Responsible for:
- Required fields
- JSON format
- Data type
- UUID format
- String length
- Basic request constraints
Example:
```go
type CreateUserRequest struct {
    Email string `json:"email" binding:"required,email"`
    Name  string `json:"name" binding:"required,min=2"`
}
```

Application Validation
Responsible for:
- Resource existence
- Workflow conditions
- Authorization orchestration
- Duplicate checks where appropriate
- Cross-aggregate process validation
Example:
```text
Check Company exists
    ↓
Check User does not already exist
    ↓
Create User
```
Domain Validation
Responsible for:
- Business rules
- Invariants
- Value Object validation
- Domain policies
Example:
```go
func (u *User) Activate() error {
    if u.Status == StatusDeleted {
        return ErrCannotActivateDeletedUser
    }

    u.Status = StatusActive

    return nil
}
```

Database Validation
Database MUST enforce data integrity using:
- Primary Key
- Foreign Key
- Unique Constraint
- NOT NULL
- CHECK Constraint
- Indexes
Application validation MUST NOT be treated as a replacement for database constraints.

## 23. Concurrency and Uniqueness
Application-level duplicate checks are useful for returning meaningful errors.

However, the database MUST remain the final authority for concurrency-safe uniqueness.

Example:
```text
Application:
Check email exists
        ↓
No existing email
        ↓
Create
        ↓
Database UNIQUE constraint
        ↓
Final protection against race condition

Do NOT rely only on:
if !exists {
    Create()
}
```

## 24. Error Handling
Errors MUST be meaningful and layered.

Example:
```text
Database Error
      ↓
Infrastructure Error
      ↓
Application Error
      ↓
HTTP Error
```
Do NOT expose:
- SQL statements
- Database credentials
- Internal database structure
- Stack traces
- Sensitive infrastructure information
to API clients.

## 25. Context
Every database operation MUST receive context.Context.

GOOD:
```go
r.db.WithContext(ctx)
```
BAD:
```go
r.db.First(...)
```
Context MUST be propagated from:
```text
HTTP Request
    ↓
Handler
    ↓
UseCase
    ↓
Repository
    ↓
GORM
```

## 26. API Response
API responses MUST use dedicated Response DTOs.

DO NOT return:
```go
models.UserModel
```
directly to the client.

Use:
```go
type UserResponse struct {
    ID   string `json:"id"`
    Name string `json:"name"`
}
```
This prevents database structure from becoming part of the public API contract.

## 27. Performance Requirements
Every API implementation MUST consider:
- Query count
- JOIN efficiency
- Index usage
- Pagination
- Selected columns
- N+1 queries
- Memory usage
- Transaction duration
- Database connection usage
The AI MUST NOT introduce unnecessary database calls.

## 28. Pagination
List APIs MUST use database-level pagination.

BAD:
```text
SELECT all records
    ↓
Load into Go
    ↓
Slice records
```
GOOD:
```text
LIMIT ?
OFFSET ?
```

or an appropriate cursor-based pagination strategy.

## 29. Repository Rules
Repositories MUST:
- Contain persistence logic
- Contain GORM queries
- Map persistence models
- Handle database-specific errors
- Accept context
- Support transaction context
Repositories MUST NOT:
- Implement HTTP logic
- Implement business workflows
- Return HTTP responses
- Contain business rules that belong to Domain
- Call external APIs

## 30. Use Case Rules
A Use Case represents a business operation.

Examples:
```text
CreateUser
GetUser
UpdateUser
DeleteUser
CreateOrganization
ApproveInvoice
PostReceipt
```
A Use Case MAY coordinate multiple repositories.

Example:
```text
CreateOrganization

CompanyRepository
BranchRepository
DepartmentRepository
UserRepository
AuditRepository
```
If these operations represent one business operation, they MUST execute within one transaction.

## 31. API Implementation Workflow
When implementing a new API, AI MUST follow this sequence:
```text
1. Understand API requirement
        ↓
2. Identify Domain entities
        ↓
3. Define business rules
        ↓
4. Define Application Input/Output
        ↓
5. Define Repository Ports
        ↓
6. Define Use Case
        ↓
7. Define GORM Models
        ↓
8. Implement GORM Repository
        ↓
9. Implement Mapper
        ↓
10. Implement Transaction if required
        ↓
11. Implement HTTP Request DTO
        ↓
12. Implement HTTP Handler
        ↓
13. Implement Response DTO
        ↓
14. Add validation
        ↓
15. Add tests
        ↓
16. Review query count and performance
```

## 32. Mandatory Review Before Completing an API
Before considering an API complete, AI MUST verify:
Architecture
- [ ] Correct module
- [ ] Correct layer
- [ ] No dependency violation
- [ ] Domain does not import GORM
- [ ] Application does not import GORM
- [ ] Presentation does not access DB
GET
- [ ] No N+1 query
- [ ] Related data uses JOIN where appropriate
- [ ] Query count minimized
- [ ] Only required columns selected
- [ ] Pagination performed in DB
- [ ] Indexes considered
CREATE
- [ ] Transaction implemented
- [ ] All related writes use same transaction
- [ ] Rollback occurs on error
- [ ] Audit handled if required
UPDATE
- [ ] Transaction implemented
- [ ] Business rules validated
- [ ] Related updates use same transaction
- [ ] Rollback occurs on error
DELETE
- [ ] Transaction implemented when state changes
- [ ] Dependency rules checked
- [ ] Soft delete used where required
- [ ] Audit handled if required
GORM
- [ ] GORM only exists in Infrastructure
- [ ] GORM Model separated from Domain Entity
- [ ] Repository implements Application Port
- [ ] Context passed to DB operations
- [ ] Transaction uses the same DB transaction instance
Security
- [ ] Authorization checked
- [ ] Sensitive data not returned
- [ ] Database errors not exposed
- [ ] Input validated
- [ ] SQL injection prevented through parameterized queries

## 33. Core Development Principles
The AI MUST follow these principles:
Presentation handles HTTP.

Application handles workflow.

Domain handles business rules.

Infrastructure handles persistence.

GORM handles database implementation.

Repository implements CRUD.

Use Case controls the business transaction boundary.

GET should minimize database round trips.

CREATE / UPDATE / DELETE must be transactional when they modify persistent state.

Database constraints are the final protection for data integrity.

Do not introduce abstractions without a concrete need.

## 34. Final Architecture
```text
┌─────────────────────────────────────────────┐
│                 Presentation                │
│                                             │
│ Handler / Request / Response / Middleware   │
└──────────────────────┬──────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────┐
│                Application                  │
│                                             │
│ Use Case / Workflow / DTO / Ports           │
│ Transaction Boundary                        │
└──────────────────────┬──────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────┐
│                   Domain                    │
│                                             │
│ Entity / Value Object / Business Rules      │
└──────────────────────┬──────────────────────┘
                       │
                       │ Ports
                       ▼
┌─────────────────────────────────────────────┐
│               Infrastructure                │
│                                             │
│ GORM Repository / Models / Mapper           │
│ Transaction Adapter                         │
└──────────────────────┬──────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────┐
│                  Database                   │
│                                             │
│ PostgreSQL / MySQL                          │
└─────────────────────────────────────────────┘
```

## 35. Non-Negotiable Rules
The following rules MUST NOT be violated without explicit architectural justification:
1. GORM MUST NOT be used outside Infrastructure.
2. Handler MUST NOT access the database directly.
3. Application MUST NOT depend on GORM.
4. Domain MUST NOT depend on Infrastructure.
5. CRUD MUST be implemented by Infrastructure Repositories.
6. Repository interfaces MUST be abstractions.
7. GET APIs MUST avoid N+1 queries.
8. Related data SHOULD be retrieved using JOINs when appropriate.
9. CREATE operations MUST use transactions.
10. UPDATE operations MUST use transactions.
11. DELETE operations that modify persistent state MUST use transactions.
12. Multiple writes belonging to one business operation MUST share the same transaction.
13. Repositories MUST NOT independently create separate transactions for the same business operation.
14. Domain business rules MUST remain in Domain.
15. Database constraints MUST enforce data integrity.
16. API responses MUST NOT expose GORM Models.
17. Every database operation MUST propagate context.
18. SQL/database implementation MUST remain inside Infrastructure.
19. External service calls MUST NOT keep database transactions open.
20. The AI MUST explain any intentional deviation from this specification.