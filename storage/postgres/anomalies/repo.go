package anomalies

import "github.com/jackc/pgx/v4/pgxpool"

// Repo — barcha qoidalar bitta joyda. Har qoida o'z faylida alohida struct;
// bu yerga qo'shilgach uning metodi Repo orqali chaqiriladi.
type Repo struct {
	*IssuedOrderInWarehouse
	*IssuedOrderPackageReturned
	*ConflictingPackageStatuses
	*ReturnedToSellerInWarehouse
	*PackageOnUnknownShelfRepo
	*DispatchedPackageOnShelf
}

func NewRepo(db *pgxpool.Pool) *Repo {
	return &Repo{
		IssuedOrderInWarehouse:      NewIssuedOrderInWarehouseRepo(db),
		IssuedOrderPackageReturned:  NewIssuedOrderPackageReturnedRepo(db),
		ConflictingPackageStatuses:  NewConflictingPackageStatusesRepo(db),
		ReturnedToSellerInWarehouse: NewReturnedToSellerInWarehouseRepo(db),
		PackageOnUnknownShelfRepo:   NewPackageOnUnknownShelfRepo(db),
		DispatchedPackageOnShelf:    NewDispatchedPackageOnShelfRepo(db),
	}
}
