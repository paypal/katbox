# v0.0.1

* Support for delayed garbage collection. Once a volume has been unmounted it is not deleted until a certain
amount of time has passed.
* Upon `NodeUnpublish` directories are unmounted but not removed from the underlying storage.
* Node driver handles calls with folders that it cannot find in memory while warning.
* Node driver now detects when a volume queued for deletion has already been deleted by an external source
and removes it from the queue.
