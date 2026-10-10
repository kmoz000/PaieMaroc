export namespace exporter {

	export class Company {
	    name: string;
	    affiliateCnss: string;
	    rib: string;
	    participant: string;

	    static createFrom(source: any = {}) {
	        return new Company(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.affiliateCnss = source["affiliateCnss"];
	        this.rib = source["rib"];
	        this.participant = source["participant"];
	    }
	}
	export class StreamResult {
	    recordCount: number;
	    totalCentimes: number;
	    fileCount: number;
	    contract: string;

	    static createFrom(source: any = {}) {
	        return new StreamResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.recordCount = source["recordCount"];
	        this.totalCentimes = source["totalCentimes"];
	        this.fileCount = source["fileCount"];
	        this.contract = source["contract"];
	    }
	}

}

export namespace main {

	export class CNSSResult {
	    path: string;
	    imported: number;
	    existingCount: number;
	    entrantCount: number;
	    employeeCount: number;
	    grossCentimes: number;
	    cappedCentimes: number;

	    static createFrom(source: any = {}) {
	        return new CNSSResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.imported = source["imported"];
	        this.existingCount = source["existingCount"];
	        this.entrantCount = source["entrantCount"];
	        this.employeeCount = source["employeeCount"];
	        this.grossCentimes = source["grossCentimes"];
	        this.cappedCentimes = source["cappedCentimes"];
	    }
	}
	export class ExportInput {
	    format: string;
	    company: exporter.Company;
	    employees: payroll.Employee[];
	    period: string;
	    executionDate: string;
	    batchNumber: number;

	    static createFrom(source: any = {}) {
	        return new ExportInput(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.format = source["format"];
	        this.company = this.convertValues(source["company"], exporter.Company);
	        this.employees = this.convertValues(source["employees"], payroll.Employee);
	        this.period = source["period"];
	        this.executionDate = source["executionDate"];
	        this.batchNumber = source["batchNumber"];
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ExportPreview {
	    name: string;
	    mediaType: string;
	    contentBase64: string;
	    recordCount: number;
	    totalCentimes: number;
	    contract: string;

	    static createFrom(source: any = {}) {
	        return new ExportPreview(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.mediaType = source["mediaType"];
	        this.contentBase64 = source["contentBase64"];
	        this.recordCount = source["recordCount"];
	        this.totalCentimes = source["totalCentimes"];
	        this.contract = source["contract"];
	    }
	}
	export class ExportResult {
	    path: string;
	    recordCount: number;
	    totalCentimes: number;
	    contract: string;

	    static createFrom(source: any = {}) {
	        return new ExportResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.recordCount = source["recordCount"];
	        this.totalCentimes = source["totalCentimes"];
	        this.contract = source["contract"];
	    }
	}
	export class LargeExportInput {
	    format: string;
	    company: exporter.Company;
	    period: string;
	    executionDate: string;
	    batchNumber: number;

	    static createFrom(source: any = {}) {
	        return new LargeExportInput(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.format = source["format"];
	        this.company = this.convertValues(source["company"], exporter.Company);
	        this.period = source["period"];
	        this.executionDate = source["executionDate"];
	        this.batchNumber = source["batchNumber"];
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace payroll {

	export class Employee {
	    id: string;
	    firstName: string;
	    lastName: string;
	    cin: string;
	    cnss: string;
	    rib: string;
	    daysWorked: number;
	    dependants: number;
	    grossCentimes: number;
	    exemptCentimes: number;
	    advanceCentimes: number;

	    static createFrom(source: any = {}) {
	        return new Employee(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.firstName = source["firstName"];
	        this.lastName = source["lastName"];
	        this.cin = source["cin"];
	        this.cnss = source["cnss"];
	        this.rib = source["rib"];
	        this.daysWorked = source["daysWorked"];
	        this.dependants = source["dependants"];
	        this.grossCentimes = source["grossCentimes"];
	        this.exemptCentimes = source["exemptCentimes"];
	        this.advanceCentimes = source["advanceCentimes"];
	    }
	}
	export class Result {
	    employee: Employee;
	    taxableGross: number;
	    employeeCnss: number;
	    employeeAmo: number;
	    professionalExpense: number;
	    incomeTax: number;
	    netPay: number;
	    employerCnss: number;
	    employerAmo: number;
	    familyAllowance: number;
	    trainingTax: number;
	    employerCost: number;
	    rulesVersion: string;

	    static createFrom(source: any = {}) {
	        return new Result(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.employee = this.convertValues(source["employee"], Employee);
	        this.taxableGross = source["taxableGross"];
	        this.employeeCnss = source["employeeCnss"];
	        this.employeeAmo = source["employeeAmo"];
	        this.professionalExpense = source["professionalExpense"];
	        this.incomeTax = source["incomeTax"];
	        this.netPay = source["netPay"];
	        this.employerCnss = source["employerCnss"];
	        this.employerAmo = source["employerAmo"];
	        this.familyAllowance = source["familyAllowance"];
	        this.trainingTax = source["trainingTax"];
	        this.employerCost = source["employerCost"];
	        this.rulesVersion = source["rulesVersion"];
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class BatchResult {
	    rows: Result[];
	    grossTotalCentimes: number;
	    netTotalCentimes: number;
	    employeeSocialTotal: number;
	    incomeTaxTotal: number;
	    employerChargesTotal: number;
	    employerCostCentimes: number;
	    rulesVersion: string;

	    static createFrom(source: any = {}) {
	        return new BatchResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rows = this.convertValues(source["rows"], Result);
	        this.grossTotalCentimes = source["grossTotalCentimes"];
	        this.netTotalCentimes = source["netTotalCentimes"];
	        this.employeeSocialTotal = source["employeeSocialTotal"];
	        this.incomeTaxTotal = source["incomeTaxTotal"];
	        this.employerChargesTotal = source["employerChargesTotal"];
	        this.employerCostCentimes = source["employerCostCentimes"];
	        this.rulesVersion = source["rulesVersion"];
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}


	export class ValidationIssue {
	    employeeId: string;
	    field: string;
	    message: string;

	    static createFrom(source: any = {}) {
	        return new ValidationIssue(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.employeeId = source["employeeId"];
	        this.field = source["field"];
	        this.message = source["message"];
	    }
	}

}

export namespace store {

	export class ImportStats {
	    imported: number;
	    rejected: number;
	    errors: string[];
	    durationMs: number;

	    static createFrom(source: any = {}) {
	        return new ImportStats(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.imported = source["imported"];
	        this.rejected = source["rejected"];
	        this.errors = source["errors"];
	        this.durationMs = source["durationMs"];
	    }
	}
	export class Page {
	    items: payroll.Employee[];
	    nextCursor: string;
	    total: number;

	    static createFrom(source: any = {}) {
	        return new Page(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], payroll.Employee);
	        this.nextCursor = source["nextCursor"];
	        this.total = source["total"];
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}
