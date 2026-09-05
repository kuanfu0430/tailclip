// 測試前後保存／還原剪貼簿的所有資料型別，不只文字。
import AppKit
import Foundation

let path = CommandLine.arguments[2]
let pasteboard = NSPasteboard.general
if CommandLine.arguments[1] == "save" {
    let items = (pasteboard.pasteboardItems ?? []).map { item in
        Dictionary(uniqueKeysWithValues: item.types.compactMap { type in
            item.data(forType: type).map { (type.rawValue, $0) }
        })
    }
    let data = try PropertyListSerialization.data(fromPropertyList: items, format: .binary, options: 0)
    guard FileManager.default.createFile(atPath: path, contents: data, attributes: [.posixPermissions: 0o600]) else {
        fatalError("無法保存測試前的剪貼簿")
    }
} else {
    let items = try PropertyListSerialization.propertyList(
        from: Data(contentsOf: URL(fileURLWithPath: path)), options: [], format: nil
    ) as! [[String: Data]]
    pasteboard.clearContents()
    pasteboard.writeObjects(items.map { values in
        let item = NSPasteboardItem()
        for (key, value) in values {
            item.setData(value, forType: NSPasteboard.PasteboardType(key))
        }
        return item
    })
}
