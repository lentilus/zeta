import XCTest
import SwiftTreeSitter
import TreeSitterMdlink

final class TreeSitterMdlinkTests: XCTestCase {
    func testCanLoadGrammar() throws {
        let parser = Parser()
        let language = Language(language: tree_sitter_mdlink())
        XCTAssertNoThrow(try parser.setLanguage(language),
                         "Error loading mdlink grammar")
    }
}
