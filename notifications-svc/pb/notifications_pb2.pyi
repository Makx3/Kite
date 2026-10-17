import datetime

from google.protobuf import timestamp_pb2 as _timestamp_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class NotificationType(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    NOTIFICATION_TYPE_UNSPECIFIED: _ClassVar[NotificationType]
    NOTIFICATION_TYPE_NEW_FOLLOWER: _ClassVar[NotificationType]
    NOTIFICATION_TYPE_LIKE: _ClassVar[NotificationType]
NOTIFICATION_TYPE_UNSPECIFIED: NotificationType
NOTIFICATION_TYPE_NEW_FOLLOWER: NotificationType
NOTIFICATION_TYPE_LIKE: NotificationType

class Notification(_message.Message):
    __slots__ = ("notification_id", "type", "user_id", "source_user_id", "post_id", "is_read", "created_at")
    NOTIFICATION_ID_FIELD_NUMBER: _ClassVar[int]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    USER_ID_FIELD_NUMBER: _ClassVar[int]
    SOURCE_USER_ID_FIELD_NUMBER: _ClassVar[int]
    POST_ID_FIELD_NUMBER: _ClassVar[int]
    IS_READ_FIELD_NUMBER: _ClassVar[int]
    CREATED_AT_FIELD_NUMBER: _ClassVar[int]
    notification_id: str
    type: NotificationType
    user_id: str
    source_user_id: str
    post_id: str
    is_read: bool
    created_at: _timestamp_pb2.Timestamp
    def __init__(self, notification_id: _Optional[str] = ..., type: _Optional[_Union[NotificationType, str]] = ..., user_id: _Optional[str] = ..., source_user_id: _Optional[str] = ..., post_id: _Optional[str] = ..., is_read: _Optional[bool] = ..., created_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class NotifyNewFollowerRequest(_message.Message):
    __slots__ = ("follower_id", "followee_id")
    FOLLOWER_ID_FIELD_NUMBER: _ClassVar[int]
    FOLLOWEE_ID_FIELD_NUMBER: _ClassVar[int]
    follower_id: str
    followee_id: str
    def __init__(self, follower_id: _Optional[str] = ..., followee_id: _Optional[str] = ...) -> None: ...

class NotifyNewFollowerResponse(_message.Message):
    __slots__ = ("notification",)
    NOTIFICATION_FIELD_NUMBER: _ClassVar[int]
    notification: Notification
    def __init__(self, notification: _Optional[_Union[Notification, _Mapping]] = ...) -> None: ...

class NotifyPostLikedRequest(_message.Message):
    __slots__ = ("post_id", "liker_id", "post_author_id")
    POST_ID_FIELD_NUMBER: _ClassVar[int]
    LIKER_ID_FIELD_NUMBER: _ClassVar[int]
    POST_AUTHOR_ID_FIELD_NUMBER: _ClassVar[int]
    post_id: str
    liker_id: str
    post_author_id: str
    def __init__(self, post_id: _Optional[str] = ..., liker_id: _Optional[str] = ..., post_author_id: _Optional[str] = ...) -> None: ...

class NotifyPostLikedResponse(_message.Message):
    __slots__ = ("notification",)
    NOTIFICATION_FIELD_NUMBER: _ClassVar[int]
    notification: Notification
    def __init__(self, notification: _Optional[_Union[Notification, _Mapping]] = ...) -> None: ...

class GetUnreadNotificationsRequest(_message.Message):
    __slots__ = ("user_id", "limit", "offset")
    USER_ID_FIELD_NUMBER: _ClassVar[int]
    LIMIT_FIELD_NUMBER: _ClassVar[int]
    OFFSET_FIELD_NUMBER: _ClassVar[int]
    user_id: str
    limit: int
    offset: int
    def __init__(self, user_id: _Optional[str] = ..., limit: _Optional[int] = ..., offset: _Optional[int] = ...) -> None: ...

class GetUnreadNotificationsResponse(_message.Message):
    __slots__ = ("notifications", "total")
    NOTIFICATIONS_FIELD_NUMBER: _ClassVar[int]
    TOTAL_FIELD_NUMBER: _ClassVar[int]
    notifications: _containers.RepeatedCompositeFieldContainer[Notification]
    total: int
    def __init__(self, notifications: _Optional[_Iterable[_Union[Notification, _Mapping]]] = ..., total: _Optional[int] = ...) -> None: ...

class MarkAsReadRequest(_message.Message):
    __slots__ = ("user_id", "notification_id")
    USER_ID_FIELD_NUMBER: _ClassVar[int]
    NOTIFICATION_ID_FIELD_NUMBER: _ClassVar[int]
    user_id: str
    notification_id: str
    def __init__(self, user_id: _Optional[str] = ..., notification_id: _Optional[str] = ...) -> None: ...

class MarkAsReadResponse(_message.Message):
    __slots__ = ("notification",)
    NOTIFICATION_FIELD_NUMBER: _ClassVar[int]
    notification: Notification
    def __init__(self, notification: _Optional[_Union[Notification, _Mapping]] = ...) -> None: ...
