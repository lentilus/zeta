package cache

type View struct {
    overlay Cache
    primary Cache
}

// func (v *View) GetExistingNotes() ([]Note, error) {
// }
//
// func (v *View) GetMissingNotes() ([]Note, error) {
// }
//
// func (v *View) GetAllNotes() ([]Note, error) {
// }
//
// func (v *View) GetForwardLinks(source Note) ([]Link, error) {
// }
//
// func (v *View) GetBackLinks(target Note) ([]Link, error) {
// }
//
// func (v *View) Subscribe() (<-chan ChangeLogEvent, error) {
// }
//
