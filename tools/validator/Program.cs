// Thin OOXML validator wrapper.
// Usage: validator <file1.docx> [file2.docx ...]
// Outputs JSON: {"file":"...","errors":[...],"warnings":[...]}
// Exit code 0 = no errors; 1 = validation errors found; 2 = usage/IO error.
//
// Every error the SDK reports counts: Schema, Semantic (duplicate ids,
// broken relationship references: the defects behind Word's "unreadable
// content"), Package and MarkupCompatibility. Warnings are reserved for
// findings the SDK itself does not classify as one of those.
using System.Text.Json;
using DocumentFormat.OpenXml.Packaging;
using DocumentFormat.OpenXml.Validation;

if (args.Length == 0)
{
    Console.Error.WriteLine("usage: validator <file> [file ...]");
    return 2;
}

var validator = new OpenXmlValidator();
int exitCode = 0;

foreach (var path in args)
{
    var errors = new List<string>();
    var warnings = new List<string>();

    try
    {
        using var doc = WordprocessingDocument.Open(path, false);
        foreach (var err in validator.Validate(doc))
        {
            var msg = $"{err.ErrorType}: {err.Description} (path: {err.Path?.XPath})";
            switch (err.ErrorType)
            {
                case ValidationErrorType.Schema:
                case ValidationErrorType.Semantic:
                case ValidationErrorType.Package:
                case ValidationErrorType.MarkupCompatibility:
                    errors.Add(msg);
                    break;
                default:
                    warnings.Add(msg);
                    break;
            }
        }
    }
    catch (Exception ex)
    {
        errors.Add($"open error: {ex.Message}");
    }

    var result = new { file = path, errors, warnings };
    Console.WriteLine(JsonSerializer.Serialize(result));

    if (errors.Count > 0)
        exitCode = 1;
}

return exitCode;
