using System;
using System.Collections.Generic;
using System.Text;
using System.Runtime.Serialization;
using System.ServiceModel.Web;
using System.Runtime.Serialization.Json;
using System.IO;

namespace DBCompiler_sql
{
    [DataContract]
    class DbField
    {
        [DataMember(IsRequired = true)]
        public string Type;
        [DataMember(IsRequired = true)]
        public Int32 Index;
        [DataMember(IsRequired = true)]
        public string Name;
        [DataMember(IsRequired = false)]
        public bool Repeated;//该字段是数组
        [DataMember(IsRequired = false)]
        public string DbDefaultValue;//如果新加了字段，加载时旧的数据使用此默认值
        [DataMember(IsRequired = false)]
        public bool Find;//生成查找方法
        [DataMember(IsRequired = false)]
        public bool Unique;//唯一性但不建立索引
        [DataMember(IsRequired = false)]
        public bool Key;//唯一并建立索引
        [DataMember(IsRequired = false)]
        public bool Incby;//不生成增量设置函数
        [DataMember(IsRequired = false)]

        public bool MinMax;//生成最小最大索引查询
    }

    [DataContract]
    class DbStruct
    {
        [DataMember(IsRequired = true)]
        public string Name;
        [DataMember(IsRequired = true)]
        public DbField[] Fields;
    }

    [DataContract]
    class DbColumn
    {
        [DataMember(IsRequired = true)]
        public string DBColumnName;
        [DataMember(IsRequired = true)]
        public bool InUse;
        [DataMember(IsRequired = true)]
        public string ColumnName;
        [DataMember(IsRequired = true)]
        public bool Preload;
        [DataMember(IsRequired = true)]
        public bool Map;
        [DataMember(IsRequired = true)]
        public bool AutoIndex;
        [DataMember(IsRequired = false)]
        public bool Simple;
        [DataMember(IsRequired = false)]
        public string SimpleType;
        [DataMember(IsRequired = false)]
        public string KeyName;
        [DataMember(IsRequired = false)]
        public string KeyType;
        [DataMember(IsRequired = true)]
        public DbField[] Fields;
        [DataMember(IsRequired = false)]
        public string DefaultValue;
        [DataMember(IsRequired = false)]
        public bool MinMax;//不生成最小最大索引查询
    }

    [DataContract]
    class DbTable
    {
        [DataMember(IsRequired = true)]
        public string TableName;
        [DataMember(IsRequired = true)]
        public string DBTableName;
        [DataMember(IsRequired = true)]
        public bool SingleRow;
        [DataMember(IsRequired = true)]
        public bool AutoPrimaryKey;
        [DataMember(IsRequired = true)]
        public string PrimaryKeyName;
        [DataMember(IsRequired = true)]
        public string PrimaryKeyType;
        [DataMember(IsRequired = true)]
        public int PrimaryKeySize;
        [DataMember(IsRequired = true)]
        public DbColumn[] Columns;
    }

    [DataContract]
    class DbDefinition
    {
        [DataMember(IsRequired = true)]
        public int Version;
        [DataMember(IsRequired = true)]
        public int SubVersion;
        [DataMember(IsRequired = true)]
        public string Namespace;
        [DataMember(IsRequired = true)]
        public DbStruct[] Structs;
        [DataMember(IsRequired = true)]
        public DbTable[] Tables;
    }

    class DbCompiler
    {
        static void CheckColumnNamesUnique(IEnumerable<DbColumn> columns)
        {
            Dictionary<string, DbColumn> dic = new Dictionary<string, DbColumn>();
            foreach (var c in columns)
            {
                if (dic.ContainsKey(c.ColumnName))
                {
                    throw new ApplicationException("重复的列名 " + c.ColumnName);
                }
                dic.Add(c.ColumnName, c);
                if (c.Fields != null && c.Fields.Length != 0)
                {
                    Dictionary<string, string> field_dic = new Dictionary<string, string>();
                    foreach (var f in c.Fields)
                    {
                        if (field_dic.ContainsKey(f.Name))
                        {
                            throw new ApplicationException("重复的字段名 " + c.ColumnName + " " + f.Name);
                        }
                        field_dic.Add(f.Name, f.Type);
                    }
                }
            }
        }
        static void CheckTableNamesUnique(IEnumerable<DbTable> tables)
        {
            Dictionary<string, DbTable> dic = new Dictionary<string, DbTable>();
            foreach (var t in tables)
            {
                if (dic.ContainsKey(t.TableName))
                {
                    throw new ApplicationException("重复的表名 " + t.TableName);
                }
                dic.Add(t.TableName, t);

                CheckColumnNamesUnique(t.Columns);
            }
        }
        static bool IsBaseType(string s)
        {
            if (s == "string" || s == "float32" || s == "int32" || s == "int16" || s == "int8" || s == "bytes" || s == "int64")
            {
                return true;
            }

            return false;
        }
        static bool IsNumberType(string s)
        {
            if (s == "float32" || s == "int32" || s == "int16" || s == "int8" || s == "int64")
            {
                return true;
            }

            return false;
        }
        static bool NeedConvert(string type)
        {
            if (GetBigType(type) == "int32" && GetBigType(type) != type)
            {
                return true;
            }

            return false;
        }
        static string GetBigType(string s)
        {
            if (s == "string")
            {
                return "string";
            }
            else if (s == "float32")
            {
                return "float32";
            }
            else if (s == "int32")
            {
                return "int32";
            }
            else if (s == "int16")
            {
                return "int32";
            }
            else if (s == "int8")
            {
                return "int32";
            }
            else if (s == "bytes")
            {
                return "bytes";
            }
            else if (s == "int64")
            {
                return "int64";
            }
            else
            {
                return s;
            }
        }
        static void CreateFile(string file, string text)
        {
            if (File.Exists(file))
            {
                File.Delete(file);
            }
            else
            {
                string dir = Path.GetDirectoryName(file);
                if (!Directory.Exists(dir))
                {
                    Directory.CreateDirectory(dir);
                }
            }
            File.AppendAllText(file, text);
        }
        static void BuildSet(StringBuilder def, string name, string type, bool repeated, string src, string dest, bool big_type)
        {
            if (repeated)
            {
                if (IsBaseType(type))
                {
                    if (type == "bytes")
                    {
                        def.AppendLine("\t" + dest + " = make([][]bytes, len(" + src + "))");
                    }
                    else
                    {
                        if (big_type)
                        {
                            def.AppendLine("\t" + dest + " = make([]" + GetBigType(type) + ", len(" + src + "))");

                        }
                        else
                        {
                            def.AppendLine("\t" + dest + " = make([]" + type + ", len(" + src + "))");

                        }
                    }
                }
                else
                {
                    def.AppendLine("\t" + dest + " = make([]" + "db" + type + "Data" + ", len(" + src + "))");
                }
                def.AppendLine("\tfor _ii, _vv := range " + src + " {");
                if (IsBaseType(type))
                {
                    if (type == "bytes")
                    {
                        def.AppendLine("\t" + dest + "[_ii] = make([]byte, len(" + src + "[_ii]))");
                        def.AppendLine("\tfor _iii, _vvv := range _vv{");
                        def.AppendLine("\t\t" + dest + "[_ii][_iii]=_vvv");
                        def.AppendLine("\t}");
                    }
                    else
                    {
                        if (NeedConvert(type))
                        {
                            if (big_type)
                            {
                                def.AppendLine("\t\t" + dest + "[_ii]=" + GetBigType(type) + "(_vv)");
                            }
                            else
                            {
                                def.AppendLine("\t\t" + dest + "[_ii]=" + type + "(_vv)");
                            }
                        }
                        else
                        {
                            def.AppendLine("\t\t" + dest + "[_ii]=_vv");
                        }
                    }
                }
                else
                {
                    def.AppendLine("\t\t_vv.clone_to(&" + dest + "[_ii])");
                }
                def.AppendLine("\t}");
            }
            else
            {
                if (IsBaseType(type))
                {
                    if (type == "bytes")
                    {
                        def.AppendLine("\t" + dest + " = make([]byte, len(" + src + "))");
                        def.AppendLine("\tfor _ii, _vv := range " + src + " {");
                        def.AppendLine("\t\t" + dest + "[_ii]=_vv");
                        def.AppendLine("\t}");
                    }
                    else
                    {
                        if (NeedConvert(type))
                        {
                            if (big_type)
                            {
                                def.AppendLine("\t" + dest + " = " + GetBigType(type) + "(" + src + ")");
                            }
                            else
                            {
                                def.AppendLine("\t" + dest + " = " + type + "(" + src + ")");
                            }

                        }
                        else
                        {
                            def.AppendLine("\t" + dest + " = " + src);
                        }
                    }
                }
                else
                {
                    def.AppendLine("\t" + src + ".clone_to(&" + dest + ")");
                }
            }
        }
        static void BuildProto(StringBuilder proto, string name, DbField[] fields, bool map)
        {
            proto.AppendLine("message " + name + "{");
            foreach (var f in fields)
            {
                if (f.Type == "float32")
                {
                    proto.AppendLine("\t" + (f.Repeated ? "repeated" : "optional") + " " + "float" + " " + f.Name + "=" + f.Index + ";");
                }
                else
                {
                    proto.AppendLine("\t" + (f.Repeated ? "repeated" : "optional") + " " + GetBigType(f.Type) + " " + f.Name + "=" + f.Index + ";");
                }
            }
            proto.AppendLine("}");
            proto.AppendLine();

            if (map)
            {
                proto.AppendLine("message " + name + "List{");
                proto.AppendLine("\t" + "repeated" + " " + name + " List=1;");
                proto.AppendLine("}");
                proto.AppendLine();
            }
        }
        static void BuildStruct(StringBuilder code, string name, DbField[] fields)
        {
            string data_name = "db" + name + "Data";

            //def
            code.AppendLine("type " + data_name + " struct{");
            foreach (var f in fields)
            {
                if (IsBaseType(f.Type))
                {
                    if (f.Type == "bytes")
                    {
                        code.AppendLine("\t" + f.Name + " " + (f.Repeated ? "[]" : "") + "[]byte");
                    }
                    else
                    {
                        code.AppendLine("\t" + f.Name + " " + (f.Repeated ? "[]" : "") + f.Type);
                    }
                }
                else
                {
                    code.AppendLine("\t" + f.Name + " " + (f.Repeated ? "[]" : "") + "db" + f.Type + "Data");
                }
            }
            code.AppendLine("}");

            //from_pb
            code.AppendLine("func (this* " + data_name + ")from_pb(pb *db." + name + "){");
            code.AppendLine("\tif pb == nil {");
            foreach (var f in fields)
            {
                if (f.Repeated)
                {
                    if (IsBaseType(f.Type))
                    {
                        if (f.Type == "bytes")
                        {
                            code.AppendLine("\t\tthis." + f.Name + " = make([]" + "[]byte" + ",0)");
                        }
                        else
                        {
                            code.AppendLine("\t\tthis." + f.Name + " = make([]" + f.Type + ",0)");
                        }
                    }
                    else
                    {
                        code.AppendLine("\t\tthis." + f.Name + " = make([]" + "db" + f.Type + "Data" + ",0)");
                    }
                }
                else
                {
                    if (IsBaseType(f.Type))
                    {
                        if (NeedConvert(f.Type))
                        {
                            if (f.DbDefaultValue != null && f.DbDefaultValue != "")
                            {
                                code.AppendLine("\t\tthis." + f.Name + " = " + f.DbDefaultValue);
                            }//no
                        }
                        else
                        {
                            if (f.DbDefaultValue != null && f.DbDefaultValue != "")
                            {
                                code.AppendLine("\t\tthis." + f.Name + " = " + f.DbDefaultValue);
                            }//no
                        }
                    }
                    else
                    {
                        code.AppendLine("\t\tthis." + f.Name + ".from_pb(nil)");
                    }
                }
            }
            code.AppendLine("\t\treturn");
            code.AppendLine("\t}");
            foreach (var f in fields)
            {
                if (f.Repeated)
                {
                    if (IsBaseType(f.Type))
                    {
                        if (f.Type == "bytes")
                        {
                            code.AppendLine("\tthis." + f.Name + " = make([]" + "[]byte" + ",len(pb.Get" + f.Name + "()))");
                        }
                        else
                        {
                            code.AppendLine("\tthis." + f.Name + " = make([]" + f.Type + ",len(pb.Get" + f.Name + "()))");
                        }
                    }
                    else
                    {
                        code.AppendLine("\tthis." + f.Name + " = make([]" + "db" + f.Type + "Data" + ",len(pb.Get" + f.Name + "()))");
                    }

                    code.AppendLine("\tfor i, v := range pb.Get" + f.Name + "() {");
                    if (IsBaseType(f.Type))
                    {
                        if (NeedConvert(f.Type))
                        {
                            code.AppendLine("\t\tthis." + f.Name + "[i] = " + f.Type + "(v)");
                        }
                        else
                        {
                            code.AppendLine("\t\tthis." + f.Name + "[i] = v");
                        }
                    }
                    else
                    {
                        code.AppendLine("\t\tthis." + f.Name + "[i].from_pb(v)");
                    }
                    code.AppendLine("\t}");
                }
                else
                {
                    if (IsBaseType(f.Type))
                    {
                        if (NeedConvert(f.Type))
                        {
                            if (f.DbDefaultValue != null && f.DbDefaultValue != "")
                            {
                                code.AppendLine("\tif pb." + f.Name + " == nil {");
                                code.AppendLine("\t\tthis." + f.Name + " = " + f.DbDefaultValue);
                                code.AppendLine("\t} else {");
                                code.AppendLine("\t\tthis." + f.Name + " = " + f.Type + "(pb.Get" + f.Name + "())");
                                code.AppendLine("\t}");
                            }
                            else
                            {
                                code.AppendLine("\tthis." + f.Name + " = " + f.Type + "(pb.Get" + f.Name + "())");
                            }
                        }
                        else
                        {
                            if (f.DbDefaultValue != null && f.DbDefaultValue != "")
                            {
                                code.AppendLine("\tif pb." + f.Name + " == nil {");
                                code.AppendLine("\t\tthis." + f.Name + " = " + f.DbDefaultValue);
                                code.AppendLine("\t} else {");
                                code.AppendLine("\t\tthis." + f.Name + " = pb.Get" + f.Name + "()");
                                code.AppendLine("\t}");
                            }
                            else
                            {
                                code.AppendLine("\tthis." + f.Name + " = pb.Get" + f.Name + "()");
                            }
                        }
                    }
                    else
                    {
                        code.AppendLine("\tthis." + f.Name + ".from_pb(pb.Get" + f.Name + "())");
                    }
                }
            }
            code.AppendLine("\treturn");
            code.AppendLine("}");

            //to_pb            
            code.AppendLine("func (this* " + data_name + ")to_pb()(pb *db." + name + "){");
            code.AppendLine("\tpb = &db." + name + "{}");
            foreach (var f in fields)
            {
                if (f.Repeated)
                {
                    if (IsBaseType(f.Type))
                    {
                        if (f.Type == "bytes")
                        {
                            code.AppendLine("\tpb." + f.Name + " = make([]" + "[]byte" + ", len(this." + f.Name + "))");
                        }
                        else
                        {
                            code.AppendLine("\tpb." + f.Name + " = make([]" + GetBigType(f.Type) + ", len(this." + f.Name + "))");
                        }
                    }
                    else
                    {
                        code.AppendLine("\tpb." + f.Name + " = make([]" + "*db." + f.Type + ", len(this." + f.Name + "))");
                    }
                    code.AppendLine("\tfor i, v := range this." + f.Name + " {");
                    if (IsBaseType(f.Type))
                    {
                        if (f.Type == "bytes")
                        {
                            code.AppendLine("\t\tpb." + f.Name + "[i]=v");
                        }
                        else
                        {
                            if (NeedConvert(f.Type))
                            {
                                code.AppendLine("\t\tpb." + f.Name + "[i]=" + GetBigType(f.Type) + "(v)");
                            }
                            else
                            {
                                code.AppendLine("\t\tpb." + f.Name + "[i]=v");
                            }
                        }
                    }
                    else
                    {
                        code.AppendLine("\t\tpb." + f.Name + "[i]=v.to_pb()");
                    }
                    code.AppendLine("\t}");
                }
                else
                {
                    if (IsBaseType(f.Type))
                    {
                        if (f.Type == "bytes")
                        {
                            code.AppendLine("\tpb." + f.Name + " = this." + f.Name);
                        }
                        else
                        {
                            if (NeedConvert(f.Type))
                            {
                                code.AppendLine("\ttemp_" + f.Name + ":=" + GetBigType(f.Type) + "(this." + f.Name + ")");
                                if (GetBigType(f.Type) == "string")
                                {
                                    code.AppendLine("\tpb." + f.Name + " = proto.String(temp_" + f.Name+")");
                                }
                                else if (GetBigType(f.Type) == "int32")
                                {
                                    code.AppendLine("\tpb." + f.Name + " = proto.Int32(temp_" + f.Name+")");
                                }
                                else if (GetBigType(f.Type) == "int64")
                                {
                                    code.AppendLine("\tpb." + f.Name + " = proto.Int64(temp_" + f.Name + ")");
                                }
                                else if (GetBigType(f.Type) == "float32")
                                {
                                    code.AppendLine("\tpb." + f.Name + " = proto.Float32(temp_" + f.Name+")");
                                }                                
                            }
                            else
                            {
                                if (f.Type == "string")
                                {
                                    code.AppendLine("\tpb." + f.Name + " = proto.String(this." + f.Name+")");
                                }
                                else if (f.Type == "int32")
                                {
                                    code.AppendLine("\tpb." + f.Name + " = proto.Int32(this." + f.Name + ")");
                                }
                                else if (f.Type == "int64")
                                {
                                    code.AppendLine("\tpb." + f.Name + " = proto.Int64(this." + f.Name + ")");
                                }
                                else if (f.Type == "float32")
                                {
                                    code.AppendLine("\tpb." + f.Name + " = proto.Float32(this." + f.Name + ")");
                                }//no                               
                            }
                        }
                    }
                    else
                    {
                        code.AppendLine("\tpb." + f.Name + " = this." + f.Name + ".to_pb()");
                    }
                }
            }
            code.AppendLine("\treturn");
            code.AppendLine("}");

            //clone
            code.AppendLine("func (this* " + data_name + ")clone_to(d *" + data_name + "){");
            foreach (var f in fields)
            {
                BuildSet(code, f.Name, f.Type, f.Repeated, "this." + f.Name, "d." + f.Name, false);
            }
            code.AppendLine("\treturn");
            code.AppendLine("}");
        }
        static void BuildColumn(DbTable t, DbColumn c, StringBuilder code, StringBuilder row_members, StringBuilder row_ctor)
        {
            //common
            string table_name = "db" + t.TableName + "Table";
            string row_name = "db" + t.TableName + "Row";
            string column_name = "db" + t.TableName + c.ColumnName + "Column";
            string data_name = "db" + t.TableName + c.ColumnName + "Data";

            if (c.Simple)
            {
                //members
                row_members.AppendLine("\tm_" + c.ColumnName + "_changed bool");
                row_members.AppendLine("\tm_" + c.ColumnName + " " + c.SimpleType);

                //get
                code.AppendLine("func (this *" + row_name + ")Get" + c.ColumnName + "( )(r " + GetBigType(c.SimpleType) + " ){");
                code.AppendLine("\tthis.m_lock.UnSafeRLock(\"" + row_name + ".Get" + column_name + "\")");
                code.AppendLine("\tdefer this.m_lock.UnSafeRUnlock()");
                code.AppendLine("\treturn " + GetBigType(c.SimpleType) + "(this.m_" + c.ColumnName + ")");
                code.AppendLine("}");

                //set
                code.AppendLine("func (this *" + row_name + ")Set" + c.ColumnName + "(v " + GetBigType(c.SimpleType) + "){");
                code.AppendLine("\tthis.m_lock.UnSafeLock(\"" + row_name + ".Set" + column_name + "\")");
                code.AppendLine("\tdefer this.m_lock.UnSafeUnlock()");
                code.AppendLine("\tthis.m_" + c.ColumnName + "=" + c.SimpleType + "(v)");
                code.AppendLine("\tthis.m_" + c.ColumnName + "_changed=true");
                code.AppendLine("\treturn");
                code.AppendLine("}");

                //inc todo

                return;
            }

            //members
            if (c.Map)
            {
                row_members.AppendLine("\t" + c.ColumnName + "s " + column_name);
            }
            else
            {
                row_members.AppendLine("\t" + c.ColumnName + " " + column_name);
            }

            //ctor
            if (c.Map)
            {
                row_ctor.AppendLine("\tthis." + c.ColumnName + "s.m_row=this");
                row_ctor.AppendLine("\tthis." + c.ColumnName + "s.m_data=make(map[" + GetBigType(c.Fields[0].Type) + "]*" + data_name + ")");
            }
            else
            {
                row_ctor.AppendLine("\tthis." + c.ColumnName + ".m_row=this");
                row_ctor.AppendLine("\tthis." + c.ColumnName + ".m_data=&" + data_name + "{}");
            }

            //column def
            code.AppendLine("type " + column_name + " struct{");
            code.AppendLine("\tm_row *" + row_name);
            if (c.Map)
            {
                code.AppendLine("\tm_data map[" + GetBigType(c.Fields[0].Type) + "]*" + data_name);
                if (c.AutoIndex)
                {
                    code.AppendLine("\tm_max_id " + c.Fields[0].Type);
                }
            }
            else
            {
                code.AppendLine("\tm_data *" + data_name);
            }
            code.AppendLine("\tm_changed bool");
            code.AppendLine("}");

            //func
            if (!c.Map)
            {
                //load
                code.AppendLine("func (this *" + column_name + ")load(data []byte)(err error){");
                code.AppendLine("\tif data == nil || len(data) == 0 {");
                code.AppendLine("\t\tthis.m_data = &"+data_name+"{}");
                code.AppendLine("\t\tthis.m_changed = false");
                code.AppendLine("\t\treturn nil");
                code.AppendLine("\t}");
                code.AppendLine("\tpb := &db." + t.TableName + c.ColumnName + "{}");
                code.AppendLine("\terr = proto.Unmarshal(data, pb)");
                code.AppendLine("\tif err != nil {");
                if (t.SingleRow)
                {
                    code.AppendLine("\t\tlog.Error(\"Unmarshal \")");
                }
                else
                {
                    code.AppendLine("\t\tlog.Error(\"Unmarshal %v\", this.m_row.Get" + t.PrimaryKeyName + "())");
                }                
                code.AppendLine("\t\treturn");
                code.AppendLine("\t}");
                code.AppendLine("\tthis.m_data = &" + data_name + "{}");
                code.AppendLine("\tthis.m_data.from_pb(pb)");
                code.AppendLine("\tthis.m_changed = false");
                code.AppendLine("\treturn");
                code.AppendLine("}");

                //save
                code.AppendLine("func (this *" + column_name + ")save( )(data []byte,err error){");
                code.AppendLine("\tpb:=this.m_data.to_pb()");
                code.AppendLine("\tdata, err = proto.Marshal(pb)");
                code.AppendLine("\tif err != nil {");
                if (t.SingleRow)
                {
                    code.AppendLine("\t\tlog.Error(\"Unmarshal \")");
                }
                else
                {
                    code.AppendLine("\t\tlog.Error(\"Marshal %v\", this.m_row.Get" + t.PrimaryKeyName + "())");
                }
                code.AppendLine("\t\treturn");
                code.AppendLine("\t}");
                code.AppendLine("\tthis.m_changed = false");
                code.AppendLine("\treturn");
                code.AppendLine("}");

                //get whole item
                code.AppendLine("func (this *" + column_name + ")Get( )(v *" + data_name + " ){");
                code.AppendLine("\tthis.m_row.m_lock.UnSafeRLock(\"" + column_name + "." + "Get" + "\")");
                code.AppendLine("\tdefer this.m_row.m_lock.UnSafeRUnlock()");
                code.AppendLine("\tv=&" + data_name + "{}");
                code.AppendLine("\tthis.m_data.clone_to(v)");
                code.AppendLine("\treturn");
                code.AppendLine("}");

                //set whole item
                code.AppendLine("func (this *" + column_name + ")Set(v " + data_name + " ){");
                code.AppendLine("\tthis.m_row.m_lock.UnSafeLock(\"" + column_name + "." + "Set" + "\")");
                code.AppendLine("\tdefer this.m_row.m_lock.UnSafeUnlock()");
                code.AppendLine("\tthis.m_data=&" + data_name + "{}");
                code.AppendLine("\tv.clone_to(this.m_data)");
                code.AppendLine("\tthis.m_changed=true");
                code.AppendLine("\treturn");
                code.AppendLine("}");

                //get set inc fields
                foreach (var f in c.Fields)
                {
                    //get         
                    if (IsBaseType(f.Type))
                    {
                        if (f.Type == "bytes")
                        {
                            code.AppendLine("func (this *" + column_name + ")Get" + f.Name + "( )(v " + (f.Repeated ? "[]" : "") + "[]byte){");
                        }
                        else
                        {
                            code.AppendLine("func (this *" + column_name + ")Get" + f.Name + "( )(v " + (f.Repeated ? "[]" : "") + GetBigType(f.Type) + " ){");
                        }
                    }
                    else
                    {
                        code.AppendLine("func (this *" + column_name + ")Get" + f.Name + "( )(v " + (f.Repeated ? "[]" : "") + "db" + f.Type + "Data" + " ){");
                    }
                    code.AppendLine("\tthis.m_row.m_lock.UnSafeRLock(\"" + column_name + "." + "Get" + f.Name + "" + "\")");
                    code.AppendLine("\tdefer this.m_row.m_lock.UnSafeRUnlock()");
                    BuildSet(code, f.Name, f.Type, f.Repeated, "this.m_data." + f.Name, "v", true);
                    code.AppendLine("\treturn");
                    code.AppendLine("}");

                    //set
                    if (IsBaseType(f.Type))
                    {
                        if (f.Type == "bytes")
                        {
                            code.AppendLine("func (this *" + column_name + ")Set" + f.Name + "(v " + (f.Repeated ? "[]" : "") + "[]byte){");
                        }
                        else
                        {
                            code.AppendLine("func (this *" + column_name + ")Set" + f.Name + "(v " + (f.Repeated ? "[]" : "") + GetBigType(f.Type) + "){");
                        }
                    }
                    else
                    {
                        code.AppendLine("func (this *" + column_name + ")Set" + f.Name + "(v " + (f.Repeated ? "[]" : "") + "db" + f.Type + "Data" + "){");
                    }
                    code.AppendLine("\tthis.m_row.m_lock.UnSafeLock(\"" + column_name + "." + "Set" + f.Name + "" + "\")");
                    code.AppendLine("\tdefer this.m_row.m_lock.UnSafeUnlock()");
                    BuildSet(code, f.Name, f.Type, f.Repeated, "v", "this.m_data." + f.Name, false);
                    code.AppendLine("\tthis.m_changed = true");
                    code.AppendLine("\treturn");
                    code.AppendLine("}");

                    //inc
                    if (IsNumberType(f.Type) && !f.Repeated && f.Incby)
                    {
                        code.AppendLine("func (this *" + column_name + ")Incby" + f.Name + "(v " + GetBigType(f.Type) + ")(r " + GetBigType(f.Type) + "){");
                        code.AppendLine("\tthis.m_row.m_lock.UnSafeLock(\"" + column_name + "." + "Incby" + f.Name + "" + "\")");
                        code.AppendLine("\tdefer this.m_row.m_lock.UnSafeUnlock()");
                        if (NeedConvert(f.Type))
                        {
                            code.AppendLine("\tthis.m_data." + f.Name + " += " + f.Type + "(v)");                            
                        }
                        else
                        {
                            code.AppendLine("\tthis.m_data." + f.Name + " += v");
                        }
                        code.AppendLine("\tthis.m_changed = true");
                        if (NeedConvert(f.Type))
                        {
                            code.AppendLine("\treturn " + GetBigType(f.Type) + "(this.m_data." + f.Name + ")");
                        }
                        else
                        {
                            code.AppendLine("\treturn this.m_data."+f.Name);
                        }                        
                        code.AppendLine("}");
                    }
                }
            }
            else//map
            {
                //load
                if (c.AutoIndex)
                {
                    code.AppendLine("func (this *" + column_name + ")load(max_id " + GetBigType(c.Fields[0].Type) + ", data []byte)(err error){");
                    code.AppendLine("\tthis.m_max_id=max_id");
                }
                else
                {
                    code.AppendLine("func (this *" + column_name + ")load(data []byte)(err error){");
                }
                code.AppendLine("\tif data == nil || len(data) == 0 {");
                code.AppendLine("\t\tthis.m_changed = false");
                code.AppendLine("\t\treturn nil");
                code.AppendLine("\t}");
                code.AppendLine("\tpb := &db." + t.TableName + c.ColumnName + "List{}");
                code.AppendLine("\terr = proto.Unmarshal(data, pb)");
                code.AppendLine("\tif err != nil {");
                code.AppendLine("\t\tlog.Error(\"Unmarshal %v\", this.m_row.Get" + t.PrimaryKeyName + "())");
                code.AppendLine("\t\treturn");
                code.AppendLine("\t}");
                code.AppendLine("\tfor _, v := range pb.List {");
                code.AppendLine("\t\td := &" + data_name + "{}");
                code.AppendLine("\t\td.from_pb(v)");
                code.AppendLine("\t\tthis.m_data[" + GetBigType(c.Fields[0].Type) + "(d." + c.Fields[0].Name + ")] = d");
                code.AppendLine("\t}");
                code.AppendLine("\tthis.m_changed = false");
                code.AppendLine("\treturn");
                code.AppendLine("}");

                //save
                if (c.AutoIndex)
                {
                    code.AppendLine("func (this *" + column_name + ")save( )(max_id " + GetBigType(c.Fields[0].Type) + ",data []byte,err error){");
                    code.AppendLine("\tmax_id=this.m_max_id");
                    code.AppendLine();
                }
                else
                {
                    code.AppendLine("func (this *" + column_name + ")save( )(data []byte,err error){");
                }
                code.AppendLine("\tpb := &db." + t.TableName + c.ColumnName + "List{}");
                code.AppendLine("\tpb.List=make([]*db." + t.TableName + c.ColumnName + ",len(this.m_data))");
                code.AppendLine("\ti:=0");
                code.AppendLine("\tfor _, v := range this.m_data {");
                code.AppendLine("\t\tpb.List[i] = v.to_pb()");
                code.AppendLine("\t\ti++");
                code.AppendLine("\t}");
                code.AppendLine("\tdata, err = proto.Marshal(pb)");
                code.AppendLine("\tif err != nil {");
                code.AppendLine("\t\tlog.Error(\"Marshal %v\", this.m_row.Get" + t.PrimaryKeyName + "())");
                code.AppendLine("\t\treturn");
                code.AppendLine("\t}");
                code.AppendLine("\tthis.m_changed = false");
                code.AppendLine("\treturn");
                code.AppendLine("}");

                //has index
                code.AppendLine("func (this *" + column_name + ")HasIndex(id " + GetBigType(c.Fields[0].Type) + ")(has bool){");
                code.AppendLine("\tthis.m_row.m_lock.UnSafeRLock(\"" + column_name + "." + "HasIndex" + "\")");
                code.AppendLine("\tdefer this.m_row.m_lock.UnSafeRUnlock()");
                code.AppendLine("\t_, has = this.m_data[id]");
                code.AppendLine("\treturn");
                code.AppendLine("}");

                //get all index
                code.AppendLine("func (this *" + column_name + ")GetAllIndex()(list []" + GetBigType(c.Fields[0].Type) + "){");
                code.AppendLine("\tthis.m_row.m_lock.UnSafeRLock(\"" + column_name + "." + "GetAllIndex" + "\")");
                code.AppendLine("\tdefer this.m_row.m_lock.UnSafeRUnlock()");
                code.AppendLine("\tlist = make([]" + GetBigType(c.Fields[0].Type) + ", len(this.m_data))");
                code.AppendLine("\ti := 0");
                code.AppendLine("\tfor k, _ := range this.m_data {");
                code.AppendLine("\t\tlist[i] = k");
                code.AppendLine("\t\ti++");
                code.AppendLine("\t}");
                code.AppendLine("\treturn");
                code.AppendLine("}");

                //get all items
                code.AppendLine("func (this *" + column_name + ")GetAll()(list []" + data_name + "){");
                code.AppendLine("\tthis.m_row.m_lock.UnSafeRLock(\"" + column_name + "." + "GetAll" + "\")");
                code.AppendLine("\tdefer this.m_row.m_lock.UnSafeRUnlock()");
                code.AppendLine("\tlist = make([]" + data_name + ", len(this.m_data))");
                code.AppendLine("\ti := 0");
                code.AppendLine("\tfor _, v := range this.m_data {");
                code.AppendLine("\t\tv.clone_to(&list[i])");
                code.AppendLine("\t\ti++");
                code.AppendLine("\t}");
                code.AppendLine("\treturn");
                code.AppendLine("}");

                //get whole item
                code.AppendLine("func (this *" + column_name + ")Get(id " + GetBigType(c.Fields[0].Type) + ")(v *" + data_name + "){");
                code.AppendLine("\tthis.m_row.m_lock.UnSafeRLock(\"" + column_name + "." + "Get" + "\")");
                code.AppendLine("\tdefer this.m_row.m_lock.UnSafeRUnlock()");
                code.AppendLine("\td := this.m_data[id]");
                code.AppendLine("\tif d==nil{");
                code.AppendLine("\t\treturn nil");
                code.AppendLine("\t}");
                code.AppendLine("\tv=&" + data_name + "{}");
                code.AppendLine("\td.clone_to(v)");
                code.AppendLine("\treturn");
                code.AppendLine("}");

                //set whole item
                code.AppendLine("func (this *" + column_name + ")Set(v " + data_name + ")(has bool){");
                //def.AppendLine("\tlog.Trace(\"%v %v\",this.m_dbp.p.Id,v." + t.Fields[0].Name + ")");
                code.AppendLine("\tthis.m_row.m_lock.UnSafeLock(\"" + column_name + "." + "Set" + "\")");
                code.AppendLine("\tdefer this.m_row.m_lock.UnSafeUnlock()");
                code.AppendLine("\td := this.m_data[" + GetBigType(c.Fields[0].Type) + "(v." + c.Fields[0].Name + ")]");
                code.AppendLine("\tif d==nil{");
                code.AppendLine("\t\tlog.Error(\"not exist %v %v\",this.m_row.Get" + t.PrimaryKeyName + "(), v." + c.Fields[0].Name + ")");
                code.AppendLine("\t\treturn false");
                code.AppendLine("\t}");
                code.AppendLine("\tv.clone_to(d)");
                code.AppendLine("\tthis.m_changed = true");
                code.AppendLine("\treturn true");
                code.AppendLine("}");

                //add whole item
                if (c.AutoIndex)
                {
                    code.AppendLine("func (this *" + column_name + ")Add(v *" + data_name + ")(id " + GetBigType(c.Fields[0].Type) + "){");
                    //def.AppendLine("\tlog.Trace(\"%v\",this.m_table.p.Id)");
                    code.AppendLine("\tthis.m_row.m_lock.UnSafeLock(\"" + column_name + "." + "Add" + "\")");
                    code.AppendLine("\tdefer this.m_row.m_lock.UnSafeUnlock()");
                    code.AppendLine("\tthis.m_max_id++");
                    code.AppendLine("\tid=this.m_max_id");
                    code.AppendLine("\tv." + c.Fields[0].Name + "=id");
                    code.AppendLine("\td:=&" + data_name + "{}");
                    code.AppendLine("\tv.clone_to(d)");
                    code.AppendLine("\tthis.m_data[v." + c.Fields[0].Name + "]=d");
                    code.AppendLine("\tthis.m_changed = true");
                    code.AppendLine("\treturn");
                    code.AppendLine("}");
                }
                else
                {
                    code.AppendLine("func (this *" + column_name + ")Add(v *" + data_name + ")(ok bool){");
                    //def.AppendLine("\tlog.Trace(\"%v %v\",this.m_table.p.Id,v." + t.Fields[0].Name + ")");
                    code.AppendLine("\tthis.m_row.m_lock.UnSafeLock(\"" + column_name + "." + "Add" + "\")");
                    code.AppendLine("\tdefer this.m_row.m_lock.UnSafeUnlock()");
                    code.AppendLine("\t_, has := this.m_data[" + GetBigType(c.Fields[0].Type) + "(v." + c.Fields[0].Name + ")]");
                    code.AppendLine("\tif has {");
                    code.AppendLine("\t\tlog.Error(\"already added %v %v\",this.m_row.Get" + t.PrimaryKeyName + "(), v." + c.Fields[0].Name + ")");
                    code.AppendLine("\t\treturn false");
                    code.AppendLine("\t}");
                    code.AppendLine("\td:=&" + data_name + "{}");
                    code.AppendLine("\tv.clone_to(d)");
                    code.AppendLine("\tthis.m_data[" + GetBigType(c.Fields[0].Type) + "(v." + c.Fields[0].Name + ")]=d");
                    code.AppendLine("\tthis.m_changed = true");
                    code.AppendLine("\treturn true");
                    code.AppendLine("}");
                }

                //remove whole item
                code.AppendLine("func (this *" + column_name + ")Remove(id " + GetBigType(c.Fields[0].Type) + "){");
                //def.AppendLine("\tlog.Trace(\"%v %v\",this.m_table.p.Id,id)");
                code.AppendLine("\tthis.m_row.m_lock.UnSafeLock(\"" + column_name + "." + "Remove" + "\")");
                code.AppendLine("\tdefer this.m_row.m_lock.UnSafeUnlock()");
                code.AppendLine("\t_, has := this.m_data[id]");
                code.AppendLine("\tif has {");
                code.AppendLine("\t\tdelete(this.m_data,id)");
                code.AppendLine("\t}");
                code.AppendLine("\tthis.m_changed = true");
                code.AppendLine("\treturn");
                code.AppendLine("}");

                //clear all items
                code.AppendLine("func (this *" + column_name + ")Clear(){");
                //def.AppendLine("\tlog.Trace(\"%v\",this.m_table.p.Id)");
                code.AppendLine("\tthis.m_row.m_lock.UnSafeLock(\"" + column_name + "." + "Clear" + "\")");
                code.AppendLine("\tdefer this.m_row.m_lock.UnSafeUnlock()");
                code.AppendLine("\tthis.m_data=make(map[" + GetBigType(c.Fields[0].Type) + "]*" + data_name + ")");
                code.AppendLine("\tthis.m_changed = true");
                code.AppendLine("\treturn");
                code.AppendLine("}");

                //NumAll
                code.AppendLine("func (this *" + column_name + ")NumAll()(n int32){");
                code.AppendLine("\tthis.m_row.m_lock.UnSafeRLock(\"" + column_name + "." + "NumAll" + "\")");
                code.AppendLine("\tdefer this.m_row.m_lock.UnSafeRUnlock()");
                code.AppendLine("\treturn int32(len(this.m_data))");
                code.AppendLine("}");

                if (c.MinMax)
                {
                    //FindMinByKey
                    code.AppendLine("func (this *" + column_name + ")FindMinByKey()(d *" + data_name + "){");
                    code.AppendLine("\tthis.m_row.m_lock.UnSafeRLock(\"" + column_name + "." + "FindMinByKey" + "\")");
                    code.AppendLine("\tdefer this.m_row.m_lock.UnSafeRUnlock()");
                    code.AppendLine("\ti := int32(65536 * 32768-1)");
                    code.AppendLine("\tvar min *" + data_name + " = nil");
                    code.AppendLine("\tfor k, v := range this.m_data {");
                    code.AppendLine("\t\tif k < i {");
                    code.AppendLine("\t\t\ti = k");
                    code.AppendLine("\t\t\tmin = v");
                    code.AppendLine("\t\t}");
                    code.AppendLine("\t}");
                    code.AppendLine("\tif min != nil {");
                    code.AppendLine("\t\td=&" + data_name + "{}");
                    code.AppendLine("\t\tmin.clone_to(d)");
                    code.AppendLine("\t}");
                    code.AppendLine("\treturn");
                    code.AppendLine("}");

                    //FindMaxByKey
                    code.AppendLine("func (this *" + column_name + ")FindMaxByKey()(d *" + data_name + "){");
                    code.AppendLine("\tthis.m_row.m_lock.UnSafeRLock(\"" + column_name + "." + "FindMaxByKey" + "\")");
                    code.AppendLine("\tdefer this.m_row.m_lock.UnSafeRUnlock()");
                    code.AppendLine("\ti := int32(-(65536 * 32768-1))");
                    code.AppendLine("\tvar max *" + data_name + " = nil");
                    code.AppendLine("\tfor k, v := range this.m_data {");
                    code.AppendLine("\t\tif k > i {");
                    code.AppendLine("\t\t\ti = k");
                    code.AppendLine("\t\t\tmax = v");
                    code.AppendLine("\t\t}");
                    code.AppendLine("\t}");
                    code.AppendLine("\tif max != nil {");
                    code.AppendLine("\t\td=&" + data_name + "{}");
                    code.AppendLine("\t\tmax.clone_to(d)");
                    code.AppendLine("\t}");
                    code.AppendLine("\treturn");
                    code.AppendLine("}");
                }

                //field methods
                for (int i = 1; i < c.Fields.Length; i++)
                {
                    var f = c.Fields[i];
                    //get         
                    if (IsBaseType(f.Type))
                    {
                        if (f.Type == "bytes")
                        {
                            code.AppendLine("func (this *" + column_name + ")Get" + f.Name + "(id " + GetBigType(c.Fields[0].Type) + ")(v " + (f.Repeated ? "[]" : "") + "[]byte,has bool){");
                        }
                        else
                        {
                            code.AppendLine("func (this *" + column_name + ")Get" + f.Name + "(id " + GetBigType(c.Fields[0].Type) + ")(v " + (f.Repeated ? "[]" : "") + GetBigType(f.Type) + " ,has bool){");
                        }
                    }
                    else
                    {
                        code.AppendLine("func (this *" + column_name + ")Get" + f.Name + "(id " + GetBigType(c.Fields[0].Type) + ")(v " + (f.Repeated ? "[]" : "") + "db" + f.Type + "Data" + ",has bool ){");
                    }
                    code.AppendLine("\tthis.m_row.m_lock.UnSafeRLock(\"" + column_name + ".Get" + f.Name + "\")");
                    code.AppendLine("\tdefer this.m_row.m_lock.UnSafeRUnlock()");
                    code.AppendLine("\td := this.m_data[id]");
                    code.AppendLine("\tif d==nil{");
                    code.AppendLine("\t\treturn");
                    code.AppendLine("\t}");
                    BuildSet(code, f.Name, f.Type, f.Repeated, "d." + f.Name, "v", true);
                    code.AppendLine("\treturn v,true");
                    code.AppendLine("}");

                    //set
                    if (IsBaseType(f.Type))
                    {
                        if (f.Type == "bytes")
                        {
                            code.AppendLine("func (this *" + column_name + ")Set" + f.Name + "(id " + GetBigType(c.Fields[0].Type) + ",v " + (f.Repeated ? "[]" : "") + "[]byte)(has bool){");
                        }
                        else
                        {
                            code.AppendLine("func (this *" + column_name + ")Set" + f.Name + "(id " + GetBigType(c.Fields[0].Type) + ",v " + (f.Repeated ? "[]" : "") + GetBigType(f.Type) + ")(has bool){");
                        }
                    }
                    else
                    {
                        code.AppendLine("func (this *" + column_name + ")Set" + f.Name + "(id " + GetBigType(c.Fields[0].Type) + ",v " + (f.Repeated ? "[]" : "") + "db" + f.Type + "Data" + ")(has bool){");
                    }
                    //def.AppendLine("\tlog.Trace(\"%v %v %v\",this.m_table.p.Id,id,v)");
                    code.AppendLine("\tthis.m_row.m_lock.UnSafeLock(\"" + column_name + ".Set" + f.Name + "\")");
                    code.AppendLine("\tdefer this.m_row.m_lock.UnSafeUnlock()");
                    code.AppendLine("\td := this.m_data[id]");
                    code.AppendLine("\tif d==nil{");
                    code.AppendLine("\t\tlog.Error(\"not exist %v %v\",this.m_row.Get" + t.PrimaryKeyName + "(), id)");
                    code.AppendLine("\t\treturn");
                    code.AppendLine("\t}");
                    BuildSet(code, f.Name, f.Type, f.Repeated, "v", "d." + f.Name, false);
                    code.AppendLine("\tthis.m_changed = true");
                    code.AppendLine("\treturn true");
                    code.AppendLine("}");

                    //inc
                    if (IsNumberType(f.Type) && !f.Repeated && f.Incby)
                    {
                        code.AppendLine("func (this *" + column_name + ")Incby" + f.Name + "(id " + GetBigType(c.Fields[0].Type) + ",v " + GetBigType(f.Type) + ")(r " + GetBigType(f.Type) + "){");
                        //def.AppendLine("\tlog.Trace(\"%v %v %v\",this.m_table.p.Id,id,v)");
                        code.AppendLine("\tthis.m_row.m_lock.UnSafeLock(\"" + column_name + ".Incby" + f.Name + "\")");
                        code.AppendLine("\tdefer this.m_row.m_lock.UnSafeUnlock()");
                        code.AppendLine("\td := this.m_data[id]");
                        code.AppendLine("\tif d==nil{");
                        code.AppendLine("\t\td = &" + data_name + "{}");
                        code.AppendLine("\t\tthis.m_data[id] = d");
                        code.AppendLine("\t}");
                        if (NeedConvert(f.Type))
                        {
                            code.AppendLine("\td." + f.Name + " += " + f.Type + "(v)");
                        }
                        else
                        {
                            code.AppendLine("\td." + f.Name + " +=  v");
                        }
                        code.AppendLine("\tthis.m_changed = true");
                        if (NeedConvert(f.Type))
                        {
                            code.AppendLine("\treturn " + GetBigType(f.Type) + "(d." + f.Name + ")");
                        }
                        else
                        {
                            code.AppendLine("\treturn d." + f.Name);
                        }                        
                        code.AppendLine("}");
                    }

                    //key
                    if (f.Key)
                    {
                        if (IsNumberType(f.Type))
                        {
                            code.AppendLine("func (this *" + column_name + ")FindBy" + f.Name + "(v " + GetBigType(f.Type) + ")(r *" + data_name + "){");
                            code.AppendLine("\tthis.m_row.m_lock.UnSafeRLock(\"" + column_name + ".FindBy" + f.Name + "\")");
                            code.AppendLine("\tdefer this.m_row.m_lock.UnSafeRUnlock()");
                            code.AppendLine("\tfor _, d := range this.m_data {");
                            if (NeedConvert(f.Type))
                            {
                                code.AppendLine("\t\tif d." + f.Name + "==" + f.Type + "(v){");
                            }
                            else
                            {
                                code.AppendLine("\t\tif d." + f.Name + "==v{");
                            }
                            code.AppendLine("\t\t\tr = &" + data_name + "{}");
                            code.AppendLine("\t\t\td.clone_to(r)");
                            code.AppendLine("\t\t\tbreak");
                            code.AppendLine("\t\t}");
                            code.AppendLine("\t}");
                            code.AppendLine("\treturn");
                            code.AppendLine("}");
                        }
                    }

                    //NumBy
                    if (IsNumberType(f.Type) && !f.Repeated && f.Find && !f.Unique && !f.Key)
                    {
                        code.AppendLine("func (this *" + column_name + ")NumBy" + f.Name + "(v " + GetBigType(f.Type) + ")(n int32){");
                        code.AppendLine("\tthis.m_row.m_lock.UnSafeRLock(\"" + column_name + ".NumBy" + f.Name + "\")");
                        code.AppendLine("\tdefer this.m_row.m_lock.UnSafeRUnlock()");
                        code.AppendLine("\tn = 0");
                        code.AppendLine("\tfor _, d := range this.m_data {");
                        if (NeedConvert(f.Type))
                        {
                            code.AppendLine("\t\tif d." + f.Name + "==" + f.Type + "(v){");
                        }
                        else
                        {
                            code.AppendLine("\t\tif d." + f.Name + "==v{");
                        }
                        code.AppendLine("\t\t\tn++");
                        code.AppendLine("\t\t}");
                        code.AppendLine("\t}");
                        code.AppendLine("\treturn");
                        code.AppendLine("}");
                    }

                    //FindAllBy
                    if (IsNumberType(f.Type) && !f.Repeated && f.Find && !f.Unique && !f.Key)
                    {
                        if (IsNumberType(f.Type))
                        {
                            code.AppendLine("func (this *" + column_name + ")FindAllBy" + f.Name + "(v " + GetBigType(f.Type) + ")(list []*" + data_name + "){");
                            code.AppendLine("\tthis.m_row.m_lock.UnSafeRLock(\"" + column_name + ".FindAllBy" + f.Name + "\")");
                            code.AppendLine("\tdefer this.m_row.m_lock.UnSafeRUnlock()");
                            code.AppendLine("\tn := 0");
                            code.AppendLine("\tfor _, d := range this.m_data {");
                            if (NeedConvert(f.Type))
                            {
                                code.AppendLine("\t\tif d." + f.Name + "==" + f.Type + "(v){");
                            }
                            else
                            {
                                code.AppendLine("\t\tif d." + f.Name + "==v{");
                            }
                            code.AppendLine("\t\t\tn++");
                            code.AppendLine("\t\t}");
                            code.AppendLine("\t}");
                            code.AppendLine("\tlist = make([]*" + data_name + ", n)");
                            code.AppendLine("\ti := 0");
                            code.AppendLine("\tfor _, d := range this.m_data {");
                            if (NeedConvert(f.Type))
                            {
                                code.AppendLine("\t\tif d." + f.Name + "==" + f.Type + "(v){");
                            }
                            else
                            {
                                code.AppendLine("\t\tif d." + f.Name + "==v{");
                            }
                            code.AppendLine("\t\tdata := &" + data_name + "{}");
                            code.AppendLine("\t\td.clone_to(data)");
                            code.AppendLine("\t\tlist[i] = data");
                            code.AppendLine("\t\ti++");
                            code.AppendLine("\t\t}");
                            code.AppendLine("\t}");
                            code.AppendLine("\treturn");
                            code.AppendLine("}");
                        }
                    }

                    //FindOneBy
                    if (IsNumberType(f.Type) && !f.Repeated && f.Find && !f.Unique && !f.Key)
                    {
                        if (IsNumberType(f.Type))
                        {
                            code.AppendLine("func (this *" + column_name + ")FindOneBy" + f.Name + "(v " + GetBigType(f.Type) + ")(r *" + data_name + "){");
                            code.AppendLine("\tthis.m_row.m_lock.UnSafeRLock(\"" + column_name + ".FindOneBy" + f.Name + "\")");
                            code.AppendLine("\tdefer this.m_row.m_lock.UnSafeRUnlock()");
                            code.AppendLine("\tfor _, d := range this.m_data {");
                            if (NeedConvert(f.Type))
                            {
                                code.AppendLine("\t\tif d." + f.Name + "==" + f.Type + "(v){");
                            }
                            else
                            {
                                code.AppendLine("\t\tif d." + f.Name + "==v{");
                            }
                            code.AppendLine("\t\t\tr = &" + data_name + "{}");
                            code.AppendLine("\t\t\td.clone_to(r)");
                            code.AppendLine("\t\t\tbreak");
                            code.AppendLine("\t\t}");
                            code.AppendLine("\t}");
                            code.AppendLine("\treturn");
                            code.AppendLine("}");
                        }
                    }

                    //FindMinBy
                    if (IsNumberType(f.Type) && !f.Repeated && f.Find && !f.Unique && !f.Key && f.MinMax)
                    {
                        code.AppendLine("func (this *" + column_name + ")FindMinBy" + f.Name + "( )(d *" + data_name + "){");
                        code.AppendLine("\tthis.m_row.m_lock.UnSafeRLock(\"" + column_name + ".FindMinBy" + f.Name + "\")");
                        code.AppendLine("\tdefer this.m_row.m_lock.UnSafeRUnlock()");
                        code.AppendLine("\ti := " + GetBigType(f.Type) + "(65536 * 32768-1)");
                        code.AppendLine("\tvar min *" + data_name + " = nil");
                        code.AppendLine("\tfor _,v  := range this.m_data {");
                        if (NeedConvert(f.Type))
                        {
                            code.AppendLine("\t\tif v." + f.Name + "<" + f.Type + "(i){");
                            code.AppendLine("\t\t\ti = " + GetBigType(f.Type) + "(v." + f.Name + ")");
                        }
                        else
                        {
                            code.AppendLine("\t\tif v." + f.Name + "<i{");
                            code.AppendLine("\t\t\ti = v." + f.Name);
                        }
                        code.AppendLine("\t\t\tmin = v");
                        code.AppendLine("\t\t}");
                        code.AppendLine("\t}");
                        code.AppendLine("\tif min != nil {");
                        code.AppendLine("\t\td=&" + data_name + "{}");
                        code.AppendLine("\t\tmin.clone_to(d)");
                        code.AppendLine("\t}");
                        code.AppendLine("\treturn");
                        code.AppendLine("}");
                    }

                    //FindMaxBy
                    if (IsNumberType(f.Type) && !f.Repeated && f.Find && !f.Unique && !f.Key && f.MinMax)
                    {
                        code.AppendLine("func (this *" + column_name + ")FindMaxBy" + f.Name + "( )(d *" + data_name + "){");
                        code.AppendLine("\tthis.m_row.m_lock.UnSafeRLock(\"" + column_name + ".FindMaxBy" + f.Name + "\")");
                        code.AppendLine("\tdefer this.m_row.m_lock.UnSafeRUnlock()");
                        code.AppendLine("\ti := " + GetBigType(f.Type) + "(-(65536 * 32768-1))");
                        code.AppendLine("\tvar max *" + data_name + " = nil");
                        code.AppendLine("\tfor _,v  := range this.m_data {");
                        if (NeedConvert(f.Type))
                        {
                            code.AppendLine("\t\tif v." + f.Name + ">" + f.Type + "(i){");
                            code.AppendLine("\t\t\ti = " + GetBigType(f.Type) + "(v." + f.Name + ")");
                        }
                        else
                        {
                            code.AppendLine("\t\tif v." + f.Name + ">i{");
                            code.AppendLine("\t\t\ti = v." + f.Name);
                        }
                        code.AppendLine("\t\t\tmax = v");
                        code.AppendLine("\t\t}");
                        code.AppendLine("\t}");
                        code.AppendLine("\tif max != nil {");
                        code.AppendLine("\t\td=&" + data_name + "{}");
                        code.AppendLine("\t\tmax.clone_to(d)");
                        code.AppendLine("\t}");
                        code.AppendLine("\treturn");
                        code.AppendLine("}");
                    }

                    //SortBy

                    //RemoveAllBy
                    if (IsNumberType(f.Type) && !f.Repeated && f.Find && !f.Unique && !f.Key)
                    {
                        code.AppendLine("func (this *" + column_name + ")RemoveAllBy" + f.Name + "(p " + GetBigType(f.Type) + "){");
                        //def.AppendLine("\tlog.Trace(\"%v %v\",this.m_table.p.Id,p)");
                        code.AppendLine("\tthis.m_row.m_lock.UnSafeLock(\"" + column_name + ".RemoveAllBy" + f.Name + "\")");
                        code.AppendLine("\tdefer this.m_row.m_lock.UnSafeUnlock()");
                        code.AppendLine("\tfor i,v:=range this.m_data{");
                        if (NeedConvert(f.Type))
                        {
                            code.AppendLine("\t\tif v." + f.Name + "==" + f.Type + "(p){");
                        }
                        else
                        {
                            code.AppendLine("\t\tif v." + f.Name + "==p{");
                        }
                        code.AppendLine("\t\t\tdelete(this.m_data,i)");
                        code.AppendLine("\t\t}");
                        code.AppendLine("\t}");
                        code.AppendLine("\tthis.m_changed = true");
                        code.AppendLine("\treturn");
                        code.AppendLine("}");
                    }

                    //RangeOpenOpen

                    //RangeOpenClose

                    //RangeCloseOpen

                    //RangeCloseClose
                }
            }
        }
        static void BuildTable(DbTable table, StringBuilder code)
        {
            if (table.PrimaryKeyType != "string" && table.PrimaryKeyType != "int32" && table.PrimaryKeyType != "int64")
            {
                throw new ApplicationException("不支持的主键类型" + table.PrimaryKeyType);
            }

            string table_name = "db" + table.TableName + "Table";
            string row_name = "db" + table.TableName + "Row";
            string db_table_name = table.DBTableName;
            if (!table.SingleRow)
            {
                db_table_name = db_table_name + "s";
            }
            string row_map_name = "map[" + table.PrimaryKeyType + "]*" + row_name;

            bool first = true;

            bool no_load = true;
            foreach (var c in table.Columns)
            {
                if (!c.InUse)
                {
                    continue;
                }

                if (!c.Preload)
                {
                    no_load = false;
                    break;
                }
            }
            bool gc = !table.SingleRow && !no_load;

            #region row
            StringBuilder row_members = new StringBuilder();
            StringBuilder row_ctor = new StringBuilder();
            foreach (var c in table.Columns)
            {
                if (c.InUse)
                {
                    BuildColumn(table, c, code, row_members, row_ctor);
                }
            }

            //row def
            code.AppendLine("type " + row_name + " struct {");
            code.AppendLine("\tm_table *" + table_name);            
            code.AppendLine("\tm_lock       *RWMutex");
            code.AppendLine("\tm_loaded  bool");
            code.AppendLine("\tm_new     bool");
            code.AppendLine("\tm_remove  bool");
            code.AppendLine("\tm_touch      int32");
            code.AppendLine("\tm_releasable bool");
            code.AppendLine("\tm_valid   bool");
			code.AppendLine("\tm_save_index	int32");
			code.AppendLine("\tm_save_release bool");
			code.AppendLine("\tm_sql_lock	*Mutex");
            code.AppendLine("\tm_" + table.PrimaryKeyName + "        " + table.PrimaryKeyType);
            code.Append(row_members);
            code.AppendLine("}");

            //row ctor
            code.AppendLine("func new_" + row_name + "(table *" + table_name + ", " + table.PrimaryKeyName + " " + table.PrimaryKeyType + ") (r *" + row_name + ") {");
            code.AppendLine("\tthis := &" + row_name + "{}");
            code.AppendLine("\tthis.m_table = table");
            code.AppendLine("\tthis.m_" + table.PrimaryKeyName + " = " + table.PrimaryKeyName);
            code.AppendLine("\tthis.m_lock = NewRWMutex()");
            code.AppendLine("\tthis.m_sql_lock = NewMutex()");
            foreach (var c in table.Columns)
            {
                if (!c.InUse)
                {
                    continue;
                }

                if (c.Simple)
                {
                    code.AppendLine("\tthis.m_" + c.ColumnName + "_changed=true");
                }
            }
            code.Append(row_ctor);
            code.AppendLine("\treturn this");
            code.AppendLine("}");

            //get primary key
            if (!table.SingleRow)
            {
                code.AppendLine("func (this *" + row_name + ") Get" + table.PrimaryKeyName + "() (r " + table.PrimaryKeyType + ") {");
                code.AppendLine("\treturn this.m_" + table.PrimaryKeyName);
                code.AppendLine("}");
            }

            #region load
			if (!table.SingleRow)
			{
				code.AppendLine("func (this *" + row_name + ") SetSaveRelease(release bool) {");
				code.AppendLine("\tthis.m_lock.UnSafeLock(\"" + row_name + ".SetSaveRelease" + "\")");
				code.AppendLine("\tdefer this.m_lock.UnSafeUnlock()");
				code.AppendLine("\tthis.m_save_release = release");
				code.AppendLine("}");
			}
            if (!table.SingleRow&&!no_load)
            {
                code.AppendLine("func (this *" + row_name + ") Load() (err error) {");
                //code.AppendLine("\tlog.Trace(\"%v\", this.PlayerId)");
                //code.AppendLine("\tthis.m_table.GC()");
				//code.AppendLine("\tthis.m_table.GCRow(this.m_" + table.PrimaryKeyName + ")");
				code.AppendLine("\tthis.m_sql_lock.UnSafeLock(\"" + row_name + ".Load" + "\")");
				code.AppendLine("\tdefer this.m_sql_lock.UnSafeUnlock()");
                code.AppendLine("\tthis.m_lock.UnSafeLock(\"" + row_name + ".Load" + "\")");
                code.AppendLine("\tdefer this.m_lock.UnSafeUnlock()");
				code.AppendLine("\tif this.m_save_release {");
				code.AppendLine("\t\tthis.m_save_release = false");
				code.AppendLine("\t}");
                code.AppendLine("\tif this.m_loaded {");
                code.AppendLine("\t\treturn");
                code.AppendLine("\t}");
                string load_scan_string = "";
                first = true;
                foreach (var c in table.Columns)
                {
                    if (!c.InUse)
                    {
                        continue;
                    }

                    if (c.Preload)
                    {
                        continue;
                    }
                    if (first)
                    {
                        first = false;
                    }
                    else
                    {
                        load_scan_string += ",";
                    }
                    if (c.Simple)
                    {
                        code.AppendLine("\tvar d" + c.ColumnName + " " + c.SimpleType);
                        load_scan_string += "&d" + c.ColumnName;
                    }
                    else
                    {
                        if (c.Map)
                        {
                            if (c.AutoIndex)
                            {
                                code.AppendLine("\tvar dMax" + c.ColumnName + "Id int32");
                                load_scan_string += "&dMax" + c.ColumnName + "Id,";
                            }
                            code.AppendLine("\tvar d" + c.ColumnName + "s []byte");
                            load_scan_string += "&d" + c.ColumnName + "s";
                        }
                        else
                        {
                            code.AppendLine("\tvar d" + c.ColumnName + " []byte");
                            load_scan_string += "&d" + c.ColumnName;
                        }
                    }
                }
                code.AppendLine("\tr := this.m_table.m_dbc.StmtQueryRow(this.m_table.m_load_select_stmt, this.m_" + table.PrimaryKeyName + ")");
                code.AppendLine("\terr = r.Scan(" + load_scan_string + ")");
                code.AppendLine("\tif err != nil {");
                code.AppendLine("\t\tlog.Error(\"scan\")");
                code.AppendLine("\t\treturn");
                code.AppendLine("\t}");
                foreach (var c in table.Columns)
                {
                    if (!c.InUse)
                    {
                        continue;
                    }

                    if (c.Preload)
                    {
                        continue;
                    }
                    if (c.Simple)
                    {
                        code.AppendLine("\t\tthis.m_" + c.ColumnName + "=d" + c.ColumnName);
                    }
                    else
                    {
                        if (c.Map)
                        {
                            if (c.AutoIndex)
                            {
                                code.AppendLine("\terr = this." + c.ColumnName + "s.load(dMax" + c.ColumnName + "Id,d" + c.ColumnName + "s)");
                            }
                            else
                            {
                                code.AppendLine("\terr = this." + c.ColumnName + "s.load(d" + c.ColumnName + "s)");
                            }
                        }
                        else
                        {
                            code.AppendLine("\terr = this." + c.ColumnName + ".load(d" + c.ColumnName + ")");
                        }

                        code.AppendLine("\tif err != nil {");
                        if (c.Map)
                        {
                            code.AppendLine("\t\tlog.Error(\"" + c.ColumnName + "s %v\", this.m_" + table.PrimaryKeyName + ")");
                        }
                        else
                        {
                            code.AppendLine("\t\tlog.Error(\"" + c.ColumnName + " %v\", this.m_" + table.PrimaryKeyName + ")");
                        }
                        code.AppendLine("\t\treturn");
                        code.AppendLine("\t}");
                    }
                }
                code.AppendLine("\tthis.m_loaded=true");
                foreach (var c in table.Columns)
                {
                    if (!c.InUse)
                    {
                        continue;
                    }

                    if (c.Preload)
                    {
                        continue;
                    }

                    if (c.Simple)
                    {
                        code.AppendLine("\tthis.m_" + c.ColumnName + "_changed=false");
                    }
                }
                code.AppendLine("\tthis.Touch(false)");
                code.AppendLine("\tatomic.AddInt32(&this.m_table.m_gc_n,1)");
                code.AppendLine("\treturn");
                code.AppendLine("}");
            }
            #endregion

            #region save data
            code.AppendLine("func (this *" + row_name + ") save_data(release bool) (err error, released bool, state int32, update_string string, args []interface{}) {");
            code.AppendLine("\tthis.m_lock.UnSafeLock(\"" + row_name + ".save_data\")");
            code.AppendLine("\tdefer this.m_lock.UnSafeUnlock()");            
            code.AppendLine("\tif this.m_new {");
            int insert_count = 1;
            foreach (var c in table.Columns)
            {
                if (!c.InUse)
                {
                    continue;
                }

                insert_count++;
                if (c.Map && c.AutoIndex)
                {
                    insert_count++;
                } 
            }
            code.AppendLine("\t\tdb_args:=new_db_args(" + insert_count + ")");
            code.AppendLine("\t\tdb_args.Push(this.m_" + table.PrimaryKeyName + ")");
            string save_insert_exec_string = "this.m_" + table.PrimaryKeyName;
            foreach (var c in table.Columns)
            {
                if (!c.InUse)
                {
                    continue;
                }

                if (c.Simple)
                {
                    code.AppendLine("\t\tdb_args.Push(this.m_" + c.ColumnName + ")");
                }
                else
                {
                    if (c.Map)
                    {
                        if (c.AutoIndex)
                        {

                            code.AppendLine("\t\tdMax" + c.ColumnName + "Id,d" + c.ColumnName + "s,db_err:=this." + c.ColumnName + "s.save()");
                        }
                        else
                        {
                            code.AppendLine("\t\td" + c.ColumnName + "s,db_err:=this." + c.ColumnName + "s.save()");
                        }
                    }
                    else
                    {
                        code.AppendLine("\t\td" + c.ColumnName + ",db_err:=this." + c.ColumnName + ".save()");
                    }
                    code.AppendLine("\t\tif db_err!=nil{");
                    code.AppendLine("\t\t\tlog.Error(\"insert save " + c.ColumnName + " failed\")");
                    code.AppendLine("\t\t\treturn db_err,false,0,\"\",nil");
                    code.AppendLine("\t\t}");
                    if (c.Map)
                    {
                        if (c.AutoIndex)
                        {
                            code.AppendLine("\t\tdb_args.Push(dMax" + c.ColumnName + "Id)");
                        }
                        code.AppendLine("\t\tdb_args.Push(d" + c.ColumnName + "s)");
                    }
                    else
                    {
                        code.AppendLine("\t\tdb_args.Push(d" + c.ColumnName + ")");
                    }
                }
            }
            code.AppendLine("\t\targs=db_args.GetArgs()");
            code.AppendLine("\t\tstate = 1");
            code.AppendLine("\t} else {");
            code.Append("\t\tif ");
            first = true;
            int update_count = 1;
            foreach (var c in table.Columns)
            {
                if (!c.InUse)
                {
                    continue;
                }

                if (first)
                {
                    first = false;
                }
                else
                {
                    code.Append("||");
                }

                if (c.Simple)
                {
                    update_count++;
                    code.Append("this.m_" + c.ColumnName + "_changed");
                }
                else
                {
                    if (c.Map)
                    {
                        if (c.AutoIndex)
                        {
                            update_count++;
                        }
                        update_count++;
                        code.Append("this." + c.ColumnName + "s.m_changed");
                    }
                    else
                    {
                        update_count++;
                        code.Append("this." + c.ColumnName + ".m_changed");
                    }
                }
            }
            code.AppendLine("{");
            code.AppendLine("\t\t\tupdate_string = \"UPDATE " + db_table_name + " SET \"");
            code.AppendLine("\t\t\tdb_args:=new_db_args(" + update_count + ")");
            foreach (var c in table.Columns)
            {
                if (!c.InUse)
                {
                    continue;
                }

                if (c.Simple)
                {
                    code.AppendLine("\t\t\tif this.m_" + c.ColumnName + "_changed{");                   
                    code.AppendLine("\t\t\t\tupdate_string+=\"" + c.ColumnName + "=?,\"");
                    code.AppendLine("\t\t\t\tdb_args.Push(this.m_" + c.ColumnName+")");
                    code.AppendLine("\t\t\t}");
                }
                else
                {
                    if (c.Map)
                    {
                        code.AppendLine("\t\t\tif this." + c.ColumnName + "s.m_changed{");                       
                        if (c.AutoIndex)
                        {
                            code.AppendLine("\t\t\t\tupdate_string+=\"Max" + c.ColumnName + "Id=?,\"");
                        }
                        code.AppendLine("\t\t\t\tupdate_string+=\"" + c.ColumnName + "s=?,\"");
                        if (c.AutoIndex)
                        {
                            code.AppendLine("\t\t\t\tdMax" + c.ColumnName + "Id,d" + c.ColumnName + "s,err:=this." + c.ColumnName + "s.save()");
                        }
                        else
                        {
                            code.AppendLine("\t\t\t\td" + c.ColumnName + "s,err:=this." + c.ColumnName + "s.save()");
                        }
                        code.AppendLine("\t\t\t\tif err!=nil{");
                        code.AppendLine("\t\t\t\t\tlog.Error(\"insert save " + c.ColumnName + " failed\")");
                        code.AppendLine("\t\t\t\t\treturn err,false,0,\"\",nil");
                        code.AppendLine("\t\t\t\t}");
                        if (c.AutoIndex)
                        {
                            code.AppendLine("\t\t\t\tdb_args.Push(dMax" + c.ColumnName + "Id)");
                        }
                        code.AppendLine("\t\t\t\tdb_args.Push(d" + c.ColumnName + "s)");
                        code.AppendLine("\t\t\t}");
                    }
                    else
                    {
                        code.AppendLine("\t\t\tif this." + c.ColumnName + ".m_changed{");
                        code.AppendLine("\t\t\t\tupdate_string+=\"" + c.ColumnName + "=?,\"");
                        code.AppendLine("\t\t\t\td" + c.ColumnName + ",err:=this." + c.ColumnName + ".save()");
                        code.AppendLine("\t\t\t\tif err!=nil{");
                        code.AppendLine("\t\t\t\t\tlog.Error(\"update save " + c.ColumnName + " failed\")");
                        code.AppendLine("\t\t\t\t\treturn err,false,0,\"\",nil");
                        code.AppendLine("\t\t\t\t}");
                        code.AppendLine("\t\t\t\tdb_args.Push(d" + c.ColumnName+")");
                        code.AppendLine("\t\t\t}");
                    }
                }
            }
            code.AppendLine("\t\t\tupdate_string = strings.TrimRight(update_string, \", \")");
            code.AppendLine("\t\t\tupdate_string+=\" WHERE " + table.PrimaryKeyName + "=?\"");
            code.AppendLine("\t\t\tdb_args.Push(this.m_" + table.PrimaryKeyName + ")");
            code.AppendLine("\t\t\targs=db_args.GetArgs()");     
            code.AppendLine("\t\t\tstate = 2");
            code.AppendLine("\t\t}");
            code.AppendLine("\t}");
            code.AppendLine("\tthis.m_new = false");
            foreach (var c in table.Columns)
            {
                if (!c.InUse)
                {
                    continue;
                }

                if (c.Simple)
                {
                    code.AppendLine("\tthis.m_" + c.ColumnName + "_changed = false");
                }
                else
                {
                    if (c.Map)
                    {
                        code.AppendLine("\tthis." + c.ColumnName + "s.m_changed = false");
                    }
                    else
                    {
                        code.AppendLine("\tthis." + c.ColumnName + ".m_changed = false");
                    }
                }
            }
            code.AppendLine("\tif this.m_save_release && this.m_loaded {");
            foreach (var c in table.Columns)
            {
                if (!c.InUse)
                {
                    continue;
                }

                if (c.Preload)
                {
                    continue;
                }

                if (c.Map)
                {
                    string data_name = "db" + table.TableName + c.ColumnName + "Data";
                    code.AppendLine("\t\tthis." + c.ColumnName + "s.m_data=make(map[" + GetBigType(c.Fields[0].Type) + "]*" + data_name + ")");
                }
            }
            if (!table.SingleRow)
            {
                code.AppendLine("\t\tatomic.AddInt32(&this.m_table.m_gc_n, -1)");
            }
            code.AppendLine("\t\tthis.m_loaded = false");
            code.AppendLine("\t\treleased = true");
			code.AppendLine("\t\tthis.m_save_release = false");
            code.AppendLine("\t}");
            code.AppendLine("\treturn nil,released,state,update_string,args");
            code.AppendLine("}");
            #endregion

            #region save
            code.AppendLine("func (this *" + row_name + ") Save(release bool) (err error, d bool, released bool) {");
			code.AppendLine("\tthis.m_sql_lock.UnSafeLock(\"" + row_name + ".Save" + "\")");
			code.AppendLine("\tdefer this.m_sql_lock.UnSafeUnlock()");
            code.AppendLine("\terr,released, state, update_string, args := this.save_data(release)");
            code.AppendLine("\tif err != nil {");
            code.AppendLine("\t\tlog.Error(\"save data failed\")");
            code.AppendLine("\t\treturn err, false, false");
            code.AppendLine("\t}");
            code.AppendLine("\tif state == 0 {");
            code.AppendLine("\t\td = false");
            code.AppendLine("\t} else if state == 1 {");
            code.AppendLine("\t\t_, err = this.m_table.m_dbc.StmtExec(this.m_table.m_save_insert_stmt, args...)");
            code.AppendLine("\t\tif err != nil {");
            code.AppendLine("\t\t\tlog.Error(\"INSERT " + db_table_name + " exec failed %v \", this.m_" + table.PrimaryKeyName + ")");
            code.AppendLine("\t\t\treturn err, false, released");
            code.AppendLine("\t\t}");
            code.AppendLine("\t\td = true");
            code.AppendLine("\t} else if state == 2 {");
            code.AppendLine("\t\t_, err = this.m_table.m_dbc.Exec(update_string, args...)");
            code.AppendLine("\t\tif err != nil {");
            code.AppendLine("\t\t\tlog.Error(\"UPDATE " + db_table_name + " exec failed %v\", this.m_" + table.PrimaryKeyName + ")");
            code.AppendLine("\t\t\treturn err, false, released");
            code.AppendLine("\t\t}");
            code.AppendLine("\t\td = true");
            code.AppendLine("\t}");
            code.AppendLine("\treturn nil, d, released");
            code.AppendLine("}");
            #endregion

            //touch
            if (!table.SingleRow)
            {
                code.AppendLine("func (this *" + row_name + ") Touch(releasable bool) {");
                code.AppendLine("\tthis.m_touch = int32(time.Now().Unix())");
                code.AppendLine("\tthis.m_releasable = releasable");
                code.AppendLine("}");
            }

            #endregion

            #region row sort
            if (!table.SingleRow)
            {
                code.AppendLine("type " + row_name + "Sort struct {");
                code.AppendLine("\trows []*" + row_name + "");
                code.AppendLine("}");
                code.AppendLine("func (this *" + row_name + "Sort) Len() (length int) {");
                code.AppendLine("\treturn len(this.rows)");
                code.AppendLine("}");
                code.AppendLine("func (this *" + row_name + "Sort) Less(i int, j int) (less bool) {");
                code.AppendLine("\treturn this.rows[i].m_touch < this.rows[j].m_touch");
                code.AppendLine("}");
                code.AppendLine("func (this *" + row_name + "Sort) Swap(i int, j int) {");
                code.AppendLine("\ttemp := this.rows[i]");
                code.AppendLine("\tthis.rows[i] = this.rows[j]");
                code.AppendLine("\tthis.rows[j] = temp");
                code.AppendLine("}");
            }
            #endregion

            #region table
            //table def
            code.AppendLine("type " + table_name + " struct{");
            code.AppendLine("\tm_dbc *DBC");
            code.AppendLine("\tm_lock *RWMutex");
            if (table.SingleRow)
            {
                code.AppendLine("\tm_row *" + row_name);
            }
            else
            {
                code.AppendLine("\tm_rows " + row_map_name);
                code.AppendLine("\tm_new_rows " + row_map_name);
                code.AppendLine("\tm_removed_rows " + row_map_name);
                code.AppendLine("\tm_gc_n int32");
                code.AppendLine("\tm_gcing int32");
                code.AppendLine("\tm_pool_size int32");
            }
            code.AppendLine("\tm_preload_select_stmt *sql.Stmt");
            if (!table.SingleRow)
            {
                if (table.PrimaryKeyType == "int64")
                {
                    code.AppendLine("\tm_preload_max_id int64");
                }
                else
                {
                    code.AppendLine("\tm_preload_max_id int32");
                }
            }           
            if (!table.SingleRow&&!no_load)
            {
                code.AppendLine("\tm_load_select_stmt *sql.Stmt");
            }
            code.AppendLine("\tm_save_insert_stmt *sql.Stmt");
            if (!table.SingleRow)
            {
                code.AppendLine("\tm_delete_stmt *sql.Stmt");
            }
            if (!table.SingleRow && table.AutoPrimaryKey)
            {
                if (table.PrimaryKeyType == "int64")
                {
                    code.AppendLine("\tm_max_id int64");
                }
                else
                {
                    code.AppendLine("\tm_max_id int32");
                }
                code.AppendLine("\tm_max_id_changed bool");
            }
			code.AppendLine("\tm_now_save_index int32");
			code.AppendLine("\tm_save_count int");
            code.AppendLine("}");

            //table ctor
            code.AppendLine("func new_" + table_name + "(dbc *DBC) (this *" + table_name + ") {");
            code.AppendLine("\tthis = &" + table_name + "{}");
            code.AppendLine("\tthis.m_dbc = dbc");
            code.AppendLine("\tthis.m_lock = NewRWMutex()");
            if (!table.SingleRow)
            {
                code.AppendLine("\tthis.m_rows = make(" + row_map_name + ")");
                code.AppendLine("\tthis.m_new_rows = make(" + row_map_name + ")");
                code.AppendLine("\tthis.m_removed_rows = make(" + row_map_name + ")");
            }
            code.AppendLine("\treturn this");
            code.AppendLine("}");

            #region table init

            #region check create table
            code.AppendLine("func (this *" + table_name + ") check_create_table() (err error) {");
            if (!table.SingleRow&&table.AutoPrimaryKey)
            {
                code.AppendLine("\t_, err = this.m_dbc.Exec(\"CREATE TABLE IF NOT EXISTS " + db_table_name + "MaxId(PlaceHolder int(11),Max" + table.PrimaryKeyName + " int(11),PRIMARY KEY (PlaceHolder))ENGINE=InnoDB ROW_FORMAT=DYNAMIC\")");
                code.AppendLine("\tif err != nil {");
                code.AppendLine("\t\tlog.Error(\"CREATE TABLE IF NOT EXISTS " + db_table_name + "MaxId failed\")");
                code.AppendLine("\t\treturn");
                code.AppendLine("\t}");

                code.AppendLine("\tr := this.m_dbc.QueryRow(\"SELECT Count(*) FROM " + db_table_name + "MaxId WHERE PlaceHolder=0\")");
                code.AppendLine("\tif r != nil {");
                code.AppendLine("\t\tvar count int32");
                code.AppendLine("\t\terr = r.Scan(&count)");
                code.AppendLine("\t\tif err != nil {");
                code.AppendLine("\t\t\tlog.Error(\"scan count failed\")");
                code.AppendLine("\t\t\treturn");
                code.AppendLine("\t\t}");
                code.AppendLine("\t\tif count == 0 {");
                code.AppendLine("\t\t_, err = this.m_dbc.Exec(\"INSERT INTO " + db_table_name + "MaxId (PlaceHolder,Max" + table.PrimaryKeyName + ") VALUES (0,0)\")");
                code.AppendLine("\t\t\tif err != nil {");
                code.AppendLine("\t\t\t\tlog.Error(\"INSERT" + db_table_name + "MaxId failed\")");
                code.AppendLine("\t\t\t\treturn");
                code.AppendLine("\t\t\t}");
                code.AppendLine("\t\t}");
                code.AppendLine("\t}");
            }
            if (table.PrimaryKeyType == "int32")
            {
                code.AppendLine("\t_, err = this.m_dbc.Exec(\"CREATE TABLE IF NOT EXISTS " + db_table_name + "(" + table.PrimaryKeyName + " int(11),PRIMARY KEY (" + table.PrimaryKeyName + "))ENGINE=InnoDB ROW_FORMAT=DYNAMIC\")");
            }
            else if (table.PrimaryKeyType == "int64")
            {
                code.AppendLine("\t_, err = this.m_dbc.Exec(\"CREATE TABLE IF NOT EXISTS " + db_table_name + "(" + table.PrimaryKeyName + " bigint(20),PRIMARY KEY (" + table.PrimaryKeyName + "))ENGINE=InnoDB ROW_FORMAT=DYNAMIC\")");
            }
            else if (table.PrimaryKeyType == "string")
            {
                code.AppendLine("\t_, err = this.m_dbc.Exec(\"CREATE TABLE IF NOT EXISTS " + db_table_name + "(" + table.PrimaryKeyName + " varchar(" + table.PrimaryKeySize + "),PRIMARY KEY (" + table.PrimaryKeyName + "))ENGINE=InnoDB ROW_FORMAT=DYNAMIC\")");
            }
            code.AppendLine("\tif err != nil {");
            code.AppendLine("\t\tlog.Error(\"CREATE TABLE IF NOT EXISTS " + db_table_name + " failed\")");
            code.AppendLine("\t\treturn");
            code.AppendLine("\t}");
            code.AppendLine("\trows, err := this.m_dbc.Query(\"SELECT COLUMN_NAME,ORDINAL_POSITION FROM information_schema.`COLUMNS` WHERE TABLE_SCHEMA=? AND TABLE_NAME='" + db_table_name + "'\", this.m_dbc.m_db_name)");
            code.AppendLine("\tif err != nil {");
            code.AppendLine("\t\tlog.Error(\"SELECT information_schema failed\")");
            code.AppendLine("\t\treturn");
            code.AppendLine("\t}");
            code.AppendLine("\tcolumns := make(map[string]int32)");
            code.AppendLine("\tfor rows.Next() {");
            code.AppendLine("\t\tvar column_name string");
            code.AppendLine("\t\tvar ordinal_position int32");
            code.AppendLine("\t\terr = rows.Scan(&column_name, &ordinal_position)");
            code.AppendLine("\t\tif err != nil {");
            code.AppendLine("\t\t\tlog.Error(\"scan information_schema row failed\")");
            code.AppendLine("\t\t\treturn");
            code.AppendLine("\t\t}");
            code.AppendLine("\t\tif ordinal_position < 1 {");
            code.AppendLine("\t\t\tlog.Error(\"col ordinal out of range\")");
            code.AppendLine("\t\t\tcontinue");
            code.AppendLine("\t\t}");
            code.AppendLine("\t\tcolumns[column_name] = ordinal_position");
            code.AppendLine("\t}");
            foreach (var c in table.Columns)
            {
                if (!c.InUse)
                {
                    continue;
                }

                if (c.Map)
                {
                    
                }
                else
                {
                    
                }
                
                if (c.Map)
                {
                    if (c.AutoIndex)
                    {
                        code.AppendLine("\t_, hasMax" + c.DBColumnName + " := columns[\"Max" + c.DBColumnName + "Id\"]");
                        code.AppendLine("\tif !hasMax" + c.DBColumnName + " {");
                        code.AppendLine("\t	_, err = this.m_dbc.Exec(\"ALTER TABLE " + db_table_name + " ADD COLUMN Max" + c.DBColumnName + "Id" + " int(11) DEFAULT 0\")");
                        code.AppendLine("\t\tif err != nil {");
                        code.AppendLine("\t\t\tlog.Error(\"ADD COLUMN map index Max" + c.DBColumnName + "Id failed\")");
                        code.AppendLine("\t\t\treturn");
                        code.AppendLine("\t\t}");
                        code.AppendLine("\t}");
                    }
                    code.AppendLine("\t_, has" + c.DBColumnName + " := columns[\"" + c.DBColumnName + "s\"]");
                    code.AppendLine("\tif !has" + c.DBColumnName + " {");
                    code.AppendLine("\t	_, err = this.m_dbc.Exec(\"ALTER TABLE " + db_table_name + " ADD COLUMN " + c.DBColumnName + "s LONGBLOB\")");
                }
                else
                {
                    code.AppendLine("\t_, has" + c.DBColumnName + " := columns[\"" + c.DBColumnName + "\"]");
                    code.AppendLine("\tif !has" + c.DBColumnName + " {");
                    if (c.Simple)
                    {
                        if (IsBaseType(c.SimpleType))
                        {
                            string big_type = GetBigType(c.SimpleType);
                            if (big_type == "bytes")
                            {
                                code.AppendLine("\t	_, err = this.m_dbc.Exec(\"ALTER TABLE " + db_table_name + " ADD COLUMN " + c.DBColumnName + " LONGBLOB\")");
                            }
                            else if (big_type == "string")
                            {
                                if (c.DefaultValue != null)
                                {
                                    code.AppendLine("\t	_, err = this.m_dbc.Exec(\"ALTER TABLE " + db_table_name + " ADD COLUMN " + c.DBColumnName + " varchar(45) DEFAULT \'" + c.DefaultValue + "\'\")");//todo
                                }
                                else
                                {
                                    code.AppendLine("\t	_, err = this.m_dbc.Exec(\"ALTER TABLE " + db_table_name + " ADD COLUMN " + c.DBColumnName + " varchar(45)\")");//todo
                                }
                            }
                            else if (big_type == "float32")
                            {
                                if (c.DefaultValue != null)
                                {
                                    code.AppendLine("\t	_, err = this.m_dbc.Exec(\"ALTER TABLE " + db_table_name + " ADD COLUMN " + c.DBColumnName + " float DEFAULT " + c.DefaultValue + "\")");
                                }
                                else
                                {
                                    code.AppendLine("\t	_, err = this.m_dbc.Exec(\"ALTER TABLE " + db_table_name + " ADD COLUMN " + c.DBColumnName + " float\")");
                                }
                            }
                            else if (big_type == "int32")
                            {
                                if (c.DefaultValue != null)
                                {
                                    code.AppendLine("\t	_, err = this.m_dbc.Exec(\"ALTER TABLE " + db_table_name + " ADD COLUMN " + c.DBColumnName + " int(11) DEFAULT " + c.DefaultValue + "\")");
                                }
                                else
                                {
                                    code.AppendLine("\t	_, err = this.m_dbc.Exec(\"ALTER TABLE " + db_table_name + " ADD COLUMN " + c.DBColumnName + " int(11)\")");
                                }
                            }
                            else if (big_type == "int64")
                            {
                                if (c.DefaultValue != null)
                                {
                                    code.AppendLine("\t	_, err = this.m_dbc.Exec(\"ALTER TABLE " + db_table_name + " ADD COLUMN " + c.DBColumnName + " bigint(20) DEFAULT " + c.DefaultValue + "\")");
                                }
                                else
                                {
                                    code.AppendLine("\t	_, err = this.m_dbc.Exec(\"ALTER TABLE " + db_table_name + " ADD COLUMN " + c.DBColumnName + " bigint(20)\")");
                                }
                            }
                        }
                        else
                        {
                            code.AppendLine("\t	_, err = this.m_dbc.Exec(\"ALTER TABLE " + db_table_name + " ADD COLUMN " + c.DBColumnName + " LONGBLOB\")");
                        }
                    }
                    else
                    {
                        code.AppendLine("\t	_, err = this.m_dbc.Exec(\"ALTER TABLE " + db_table_name + " ADD COLUMN " + c.DBColumnName + " LONGBLOB\")");
                    }
                }
                code.AppendLine("\t\tif err != nil {");
                if (c.Map)
                {
                    code.AppendLine("\t\t\tlog.Error(\"ADD COLUMN " + c.DBColumnName + "s failed\")");
                }
                else
                {
                    code.AppendLine("\t\t\tlog.Error(\"ADD COLUMN " + c.DBColumnName + " failed\")");
                }

                code.AppendLine("\t\t\treturn");
                code.AppendLine("\t\t}");
                code.AppendLine("\t}");
            }
            code.AppendLine("\treturn");
            code.AppendLine("}");
            #endregion

            #region prepare preload select stmt
            string preload_select_string = "";
            if (table.SingleRow)
            {
                preload_select_string += "\"SELECT ";
                foreach (var c in table.Columns)
                {
                    if (!c.InUse)
                    {
                        continue;
                    }
                    if (c.Map)
                    {
                        if (c.AutoIndex)
                        {
                            preload_select_string += "Max" + c.ColumnName + "Id,";
                        }
                        preload_select_string += c.ColumnName + "s,";
                    }
                    else
                    {
                        preload_select_string += c.ColumnName + ",";
                    }
                }
                preload_select_string = preload_select_string.TrimEnd(new char[] { ',' });
                preload_select_string += " FROM " + db_table_name + " WHERE " + table.PrimaryKeyName + "=0\"";
            }
            else
            {
                preload_select_string += "\"SELECT " + table.PrimaryKeyName;
                foreach (var c in table.Columns)
                {
                    if (!c.InUse)
                    {
                        continue;
                    }
                    if (!c.Preload)
                    {
                        continue;
                    }
                    if (c.Map)
                    {
                        if (c.AutoIndex)
                        {
                            preload_select_string += "," + "Max" + c.ColumnName + "Id";
                        }
                        preload_select_string += "," + c.ColumnName + "s";
                    }
                    else
                    {
                        preload_select_string += "," + c.ColumnName;
                    }
                }
                preload_select_string += " FROM " + db_table_name + "\"";
            }
            code.AppendLine("func (this *" + table_name + ") prepare_preload_select_stmt() (err error) {");
            code.AppendLine("\tthis.m_preload_select_stmt,err=this.m_dbc.StmtPrepare(" + preload_select_string + ")");
            code.AppendLine("\tif err!=nil{");
            code.AppendLine("\t\tlog.Error(\"prepare failed\")");
            code.AppendLine("\t\treturn");
            code.AppendLine("\t}");
            code.AppendLine("\treturn");
            code.AppendLine("}");
            #endregion

            #region prepare load select stmt
            if (!table.SingleRow&&!no_load)
            {
                string load_select_string = "\"SELECT ";
                first = true;
                foreach (var c in table.Columns)
                {
                    if (!c.InUse)
                    {
                        continue;
                    }

                    if (c.Preload)
                    {
                        continue;
                    }
                    if (first)
                    {
                        first = false;
                    }
                    else
                    {
                        load_select_string += ",";
                    }
                    if (c.Map)
                    {
                        if (c.AutoIndex)
                        {
                            load_select_string += "Max" + c.ColumnName + "Id,";
                        }
                        load_select_string += c.ColumnName + "s";
                    }
                    else
                    {
                        load_select_string += c.ColumnName;
                    }
                }

                load_select_string += " FROM " + db_table_name + " WHERE " + table.PrimaryKeyName + "=?\"";
                code.AppendLine("func (this *" + table_name + ") prepare_load_select_stmt() (err error) {");
                code.AppendLine("\tthis.m_load_select_stmt,err=this.m_dbc.StmtPrepare(" + load_select_string + ")");
                code.AppendLine("\tif err!=nil{");
                code.AppendLine("\t\tlog.Error(\"prepare failed\")");
                code.AppendLine("\t\treturn");
                code.AppendLine("\t}");
                code.AppendLine("\treturn");
                code.AppendLine("}");
            }
            #endregion

            #region prepare save insert stmt
            string save_insert_stmt_string = "\"INSERT INTO " + db_table_name + " (" + table.PrimaryKeyName + "";
            string save_insert_stmt_values_string = "?";
            foreach (var c in table.Columns)
            {
                if (!c.InUse)
                {
                    continue;
                }

                save_insert_stmt_values_string += ",?";
                if (c.Simple)
                {
                    save_insert_stmt_string += "," + c.ColumnName;
                }
                else
                {
                    if (c.Map)
                    {
                        if (c.AutoIndex)
                        {
                            save_insert_stmt_values_string += ",?";
                            save_insert_stmt_string += ",Max" + c.ColumnName + "Id," + c.ColumnName + "s";
                        }
                        else
                        {
                            save_insert_stmt_string += "," + c.ColumnName + "s";
                        }
                    }
                    else
                    {
                        save_insert_stmt_string += "," + c.ColumnName;
                    }
                }
            }
            save_insert_stmt_string += ") VALUES (" + save_insert_stmt_values_string + ")\"";
            code.AppendLine("func (this *" + table_name + ") prepare_save_insert_stmt()(err error){");
            code.AppendLine("\tthis.m_save_insert_stmt,err=this.m_dbc.StmtPrepare(" + save_insert_stmt_string + ")");
            code.AppendLine("\tif err!=nil{");
            code.AppendLine("\t\tlog.Error(\"prepare failed\")");
            code.AppendLine("\t\treturn");
            code.AppendLine("\t}");
            code.AppendLine("\treturn");
            code.AppendLine("}");
            #endregion

            #region prepare delete stmt
            if (!table.SingleRow)
            {
                code.AppendLine("func (this *" + table_name + ") prepare_delete_stmt() (err error) {");
                code.AppendLine("\tthis.m_delete_stmt,err=this.m_dbc.StmtPrepare(\"DELETE FROM " + db_table_name + " WHERE " + table.PrimaryKeyName + "=?\")");
                code.AppendLine("\tif err!=nil{");
                code.AppendLine("\t\tlog.Error(\"prepare failed\")");
                code.AppendLine("\t\treturn");
                code.AppendLine("\t}");
                code.AppendLine("\treturn");
                code.AppendLine("}");
            }
            #endregion

            //Init
            code.AppendLine("func (this *" + table_name + ") Init() (err error) {");
            code.AppendLine("\terr=this.check_create_table()");
            code.AppendLine("\tif err!=nil{");
            code.AppendLine("\t\tlog.Error(\"check_create_table failed\")");
            code.AppendLine("\t\treturn");
            code.AppendLine("\t}");
            code.AppendLine("\terr=this.prepare_preload_select_stmt()");
            code.AppendLine("\tif err!=nil{");
            code.AppendLine("\t\tlog.Error(\"prepare_preload_select_stmt failed\")");
            code.AppendLine("\t\treturn");
            code.AppendLine("\t}");
            if (!table.SingleRow&&!no_load)
            {
                code.AppendLine("\terr=this.prepare_load_select_stmt()");
                code.AppendLine("\tif err!=nil{");
                code.AppendLine("\t\tlog.Error(\"prepare_load_select_stmt failed\")");
                code.AppendLine("\t\treturn");
                code.AppendLine("\t}");
            }
            code.AppendLine("\terr=this.prepare_save_insert_stmt()");
            code.AppendLine("\tif err!=nil{");
            code.AppendLine("\t\tlog.Error(\"prepare_save_insert_stmt failed\")");
            code.AppendLine("\t\treturn");
            code.AppendLine("\t}");
            if (!table.SingleRow)
            {
                code.AppendLine("\terr=this.prepare_delete_stmt()");
                code.AppendLine("\tif err!=nil{");
                code.AppendLine("\t\tlog.Error(\"prepare_save_insert_stmt failed\")");
                code.AppendLine("\t\treturn");
                code.AppendLine("\t}");
            }
			code.AppendLine("\tthis.m_now_save_index = 0");
			code.AppendLine("\tthis.m_save_count = 0");
            code.AppendLine("\treturn");
            code.AppendLine("}");
            #endregion

            #region preload
            code.AppendLine("func (this *" + table_name + ") Preload() (err error) {");
            if (table.SingleRow)
            {
                code.AppendLine("\tr := this.m_dbc.StmtQueryRow(this.m_preload_select_stmt)");
            }
            else
            {
                if (table.AutoPrimaryKey)
                {
                    code.AppendLine("\tr_max_id := this.m_dbc.QueryRow(\"SELECT Max" + table.PrimaryKeyName + " FROM " + db_table_name + "MaxId WHERE PLACEHOLDER=0\")");
                    code.AppendLine("\tif r_max_id != nil {");
                    code.AppendLine("\t\terr = r_max_id.Scan(&this.m_max_id)");
                    code.AppendLine("\t\tif err != nil {");
                    code.AppendLine("\t\t\tlog.Error(\"scan max id failed\")");
                    code.AppendLine("\t\t\treturn");
                    code.AppendLine("\t\t}");
                    code.AppendLine("\t}");
                }
                code.AppendLine("\tr, err := this.m_dbc.StmtQuery(this.m_preload_select_stmt)");
                code.AppendLine("\tif err != nil {");
                code.AppendLine("\t\tlog.Error(\"SELECT\")");
                code.AppendLine("\t\treturn");
                code.AppendLine("\t}");
            }

            string preload_scan_string = "";
            if (!table.SingleRow)
            {
                code.AppendLine("\tvar " + table.PrimaryKeyName + " " + table.PrimaryKeyType);
                preload_scan_string += "&" + table.PrimaryKeyName + ",";
            }           
            foreach (var c in table.Columns)
            {
                if (!c.InUse)
                {
                    continue;
                }
                if (!c.Preload)
                {
                    continue;
                }
                if (c.Simple)
                {
                    code.AppendLine("\tvar d" + c.ColumnName + " " + c.SimpleType);
                    preload_scan_string += "&d" + c.ColumnName+",";
                }
                else
                {
                    if (c.Map)
                    {
                        if (c.AutoIndex)
                        {
                            code.AppendLine("\tvar dMax" + c.ColumnName + "Id int32");
                            preload_scan_string += "&dMax" + c.ColumnName + "Id,";
                        }
                        code.AppendLine("\tvar d" + c.ColumnName + "s []byte");
                        preload_scan_string += "&d" + c.ColumnName + "s,";
                    }
                    else
                    {
                        code.AppendLine("\tvar d" + c.ColumnName + " []byte");
                        preload_scan_string += "&d" + c.ColumnName+",";
                    }
                }
            }
            if ((table.PrimaryKeyType == "int32" || table.PrimaryKeyType == "int64") && !table.SingleRow && !table.AutoPrimaryKey)
            {
                code.AppendLine("\t\tthis.m_preload_max_id = 0");          
            }
            preload_scan_string = preload_scan_string.TrimEnd(new char[] { ',' });
            if (table.SingleRow)
            {
                code.AppendLine("\terr = r.Scan(" + preload_scan_string + ")");
                code.AppendLine("\tif err!=nil{");
                code.AppendLine("\t\tif err!=sql.ErrNoRows{");
                code.AppendLine("\t\t\tlog.Error(\"Scan failed\")");
                code.AppendLine("\t\t\treturn");
                code.AppendLine("\t\t}");
                code.AppendLine("\t}else{");
            }
            else
            {
                code.AppendLine("\tfor r.Next() {");
                code.AppendLine("\t\terr = r.Scan(" + preload_scan_string + ")");
                code.AppendLine("\t\tif err != nil {");
                code.AppendLine("\t\t\tlog.Error(\"Scan\")");
                code.AppendLine("\t\t\treturn");
                code.AppendLine("\t\t}");
                if (table.PrimaryKeyType == "int32")
                {
                    if (table.AutoPrimaryKey)
                    {
                        code.AppendLine("\t\tif " + table.PrimaryKeyName + ">this.m_max_id{");
                        code.AppendLine("\t\t\tlog.Error(\"max id ext\")");
                        code.AppendLine("\t\t\tthis.m_max_id = " + table.PrimaryKeyName + "");
                        code.AppendLine("\t\t\tthis.m_max_id_changed = true");
                        code.AppendLine("\t\t}");
                    }
                    else
                    {
                        code.AppendLine("\t\tif " + table.PrimaryKeyName + ">this.m_preload_max_id{");
                        code.AppendLine("\t\t\tthis.m_preload_max_id =" + table.PrimaryKeyName + "");
                        code.AppendLine("\t\t}");
                    }
                }

                if (table.PrimaryKeyType == "int64")
                {
                    if (table.AutoPrimaryKey)
                    {
                        code.AppendLine("\t\tif " + table.PrimaryKeyName + ">this.m_max_id{");
                        code.AppendLine("\t\t\tlog.Error(\"max id ext\")");
                        code.AppendLine("\t\t\tthis.m_max_id = " + table.PrimaryKeyName + "");
                        code.AppendLine("\t\t\tthis.m_max_id_changed = true");
                        code.AppendLine("\t\t}");
                    }
                    else
                    {
                        code.AppendLine("\t\tif " + table.PrimaryKeyName + ">this.m_preload_max_id{");
                        code.AppendLine("\t\t\tthis.m_preload_max_id =" + table.PrimaryKeyName + "");
                        code.AppendLine("\t\t}");
                    }
                }
            }
            if (table.SingleRow)
            {
                code.AppendLine("\t\trow := new_" + row_name + "(this,0)");
            }
            else
            {
                code.AppendLine("\t\trow := new_" + row_name + "(this," + table.PrimaryKeyName + ")");
            }            
            foreach (var c in table.Columns)
            {
                if (!c.InUse)
                {
                    continue;
                }
                if (!table.SingleRow&&!c.Preload)
                {
                    continue;
                }
                if (c.Simple)
                {
                    code.AppendLine("\t\trow.m_" + c.ColumnName + "=d" + c.ColumnName);
                }
                else
                {
                    if (c.Map)
                    {
                        if (c.AutoIndex)
                        {
                            code.AppendLine("\t\terr = row." + c.ColumnName + "s.load(dMax" + c.ColumnName + "Id,d" + c.ColumnName + "s)");
                        }
                        else
                        {
                            code.AppendLine("\t\terr = row." + c.ColumnName + "s.load(d" + c.ColumnName + "s)");
                        }
                    }
                    else
                    {
                        code.AppendLine("\t\terr = row." + c.ColumnName + ".load(d" + c.ColumnName + ")");
                    }

                    code.AppendLine("\t\tif err != nil {");
                    if (table.SingleRow)
                    {
                        code.AppendLine("\t\t\tlog.Error(\"" + c.ColumnName + " \")");
                    }
                    else
                    {
                        if (c.Map)
                        {
                            code.AppendLine("\t\t\tlog.Error(\"" + c.ColumnName + "s %v\", " + table.PrimaryKeyName + ")");
                        }
                        else
                        {
                            code.AppendLine("\t\t\tlog.Error(\"" + c.ColumnName + " %v\", " + table.PrimaryKeyName + ")");
                        }
                    }
                    code.AppendLine("\t\t\treturn");
                    code.AppendLine("\t\t}");
                }
            }
            foreach (var c in table.Columns)
            {
                if (!c.InUse)
                {
                    continue;
                }

                if (!table.SingleRow&&!c.Preload)
                {
                    continue;
                }

                if (c.Simple)
                {
                    code.AppendLine("\t\trow.m_" + c.ColumnName + "_changed=false");
                }
			}
			code.AppendLine("\t\trow.m_valid = true");
			code.AppendLine("\t\trow.m_save_index = int32(this.m_save_count % config.DBCST_MIN)");
			code.AppendLine("\t\tthis.m_save_count++");
            if (table.SingleRow)
            {
                code.AppendLine("\t\trow.m_loaded=true");
                code.AppendLine("\t\tthis.m_row=row");
                code.AppendLine("\t}");
                code.AppendLine("\tif this.m_row == nil {");
                code.AppendLine("\t\tthis.m_row = new_" + row_name + "(this, 0)");
                code.AppendLine("\t\tthis.m_row.m_new = true");
                code.AppendLine("\t\tthis.m_row.m_valid = true");
                code.AppendLine("\t\terr = this.Save(false, true)");
                code.AppendLine("\t\tif err != nil {");
                code.AppendLine("\t\t\tlog.Error(\"save failed\")");
                code.AppendLine("\t\t\treturn");
                code.AppendLine("\t\t}");
                code.AppendLine("\t\tthis.m_row.m_loaded = true");
                code.AppendLine("\t}");                
            }
            else
            {
                code.AppendLine("\t\tthis.m_rows[" + table.PrimaryKeyName + "]=row");
                code.AppendLine("\t}");
            }
            code.AppendLine("\treturn");
            code.AppendLine("}");
            //preloaded max id
            if (!table.SingleRow)
            {
                if (table.PrimaryKeyType == "int64")
                {
                    code.AppendLine("func (this *" + table_name + ") GetPreloadedMaxId() (max_id int64) {");
                }
                else
                {
                    code.AppendLine("func (this *" + table_name + ") GetPreloadedMaxId() (max_id int32) {");
                }
                
                code.AppendLine("\treturn this.m_preload_max_id");
                code.AppendLine("}");
            }
            #endregion

            if (table.SingleRow)
            {
                //save
				code.AppendLine("func (this *" + table_name + ") Save(quick bool, save_all bool) (err error) {");
                code.AppendLine("\tif this.m_row==nil{");
                code.AppendLine("\t\treturn errors.New(\"row nil\")");
                code.AppendLine("\t}");
				code.AppendLine("\tif !save_all && this.m_row.m_save_index != this.m_now_save_index % int32(config.DBCST_MIN) {");
				code.AppendLine("\t\tthis.m_now_save_index++");
				code.AppendLine("\t\treturn");
				code.AppendLine("\t}");
				code.AppendLine("\terr, _, _ = this.m_row.Save(false)");
                code.AppendLine("\treturn err");
                code.AppendLine("}");

                //get row
                code.AppendLine("func (this *" + table_name + ") GetRow( ) (row *" + row_name + ") {");
                code.AppendLine("\treturn this.m_row");
                code.AppendLine("}");
            }
            else
            {
                //fetch rows                
                code.AppendLine("func (this *" + table_name + ") fetch_rows(rows " + row_map_name + ", save_all bool) (r " + row_map_name + ") {");
                code.AppendLine("\tthis.m_lock.UnSafeLock(\"" + table_name + ".fetch_rows\")");
                code.AppendLine("\tdefer this.m_lock.UnSafeUnlock()");
                code.AppendLine("\tr = make(" + row_map_name + ")");
				code.AppendLine("\tnow_save_index := this.m_now_save_index % int32(config.DBCST_MIN)");
                code.AppendLine("\tfor i, v := range rows {");
				code.AppendLine("\t\tif !save_all && v.m_save_index != now_save_index {");
				code.AppendLine("\t\t\tcontinue");
				code.AppendLine("\t\t}");
                code.AppendLine("\t\tr[i] = v");
                code.AppendLine("\t}");
                code.AppendLine("\treturn r");
                code.AppendLine("}");

                //fetch_new_rows
				code.AppendLine("func (this *" + table_name + ") fetch_new_rows(save_all bool) (new_rows " + row_map_name + ") {");
                code.AppendLine("\tthis.m_lock.UnSafeLock(\"" + table_name + ".fetch_new_rows\")");
                code.AppendLine("\tdefer this.m_lock.UnSafeUnlock()");
                code.AppendLine("\tnew_rows = make(" + row_map_name + ")");
				code.AppendLine("\tnow_save_index := this.m_now_save_index % int32(config.DBCST_MIN)");
                code.AppendLine("\tfor i, v := range this.m_new_rows {");
                code.AppendLine("\t\t_, has := this.m_rows[i]");
                code.AppendLine("\t\tif has {");
                code.AppendLine("\t\t\tlog.Error(\"rows already has new rows %v\", i)");
                code.AppendLine("\t\t\tcontinue");
                code.AppendLine("\t\t}");
				code.AppendLine("\t\tif !save_all && v.m_save_index != now_save_index {");
				code.AppendLine("\t\t\tcontinue");
				code.AppendLine("\t\t}");
                code.AppendLine("\t\tthis.m_rows[i] = v");
                code.AppendLine("\t\tnew_rows[i] = v");
                code.AppendLine("\t}");
                code.AppendLine("\tfor i, _ := range new_rows {");
                code.AppendLine("\t\tdelete(this.m_new_rows, i)");
                code.AppendLine("\t}");
                code.AppendLine("\treturn");
                code.AppendLine("}");

                //save_rows
                code.AppendLine("func (this *" + table_name + ") save_rows(rows " + row_map_name + ", quick bool) {");
                code.AppendLine("\tfor _, v := range rows {");
                code.AppendLine("\t\tif this.m_dbc.m_quit && !quick {");
                code.AppendLine("\t\t\treturn");
                code.AppendLine("\t\t}");      
                code.AppendLine("\t\terr, delay, _ := v.Save(false)");
                code.AppendLine("\t\tif err != nil {");
                code.AppendLine("\t\t\tlog.Error(\"save failed %v\", err)");
                code.AppendLine("\t\t}");
                code.AppendLine("\t\tif this.m_dbc.m_quit && !quick {");
                code.AppendLine("\t\t\treturn");
                code.AppendLine("\t\t}");
                code.AppendLine("\t\tif delay&&!quick {");
                code.AppendLine("\t\t\ttime.Sleep(time.Millisecond * 50)");
                code.AppendLine("\t\t}");
                code.AppendLine("\t}");
                code.AppendLine("}");

                //save
                code.AppendLine("func (this *" + table_name + ") Save(quick bool, save_all bool) (err error){");
                if (table.AutoPrimaryKey)
                {
                    code.AppendLine("\tif this.m_max_id_changed {");
                    code.AppendLine("\t\tmax_id := atomic.LoadInt32(&this.m_max_id)");
                    code.AppendLine("\t\t_, err := this.m_dbc.Exec(\"UPDATE " + db_table_name + "MaxId SET Max" + table.PrimaryKeyName + "=?\", max_id)");
                    code.AppendLine("\t\tif err != nil {");
                    code.AppendLine("\t\t\tlog.Error(\"save max id failed %v\", err)");
                    code.AppendLine("\t\t}");
                    code.AppendLine("\t}");
                }
				code.AppendLine("\tremoved_rows := this.fetch_rows(this.m_removed_rows, save_all)");
                code.AppendLine("\tfor _, v := range removed_rows {");
                code.AppendLine("\t\t_, err := this.m_dbc.StmtExec(this.m_delete_stmt, v.Get" + table.PrimaryKeyName + "())");
                code.AppendLine("\t\tif err != nil {");
                code.AppendLine("\t\t\tlog.Error(\"exec delete stmt failed %v\", err)");
                code.AppendLine("\t\t}");
                code.AppendLine("\t\tv.m_valid = false");
                code.AppendLine("\t\tif !quick {");
                code.AppendLine("\t\t\ttime.Sleep(time.Millisecond * 5)");
                code.AppendLine("\t\t}");
                code.AppendLine("\t}");
                //code.AppendLine("\tthis.m_removed_rows = make(" + row_map_name + ")");
				code.AppendLine("\tfor k := range removed_rows {");
				code.AppendLine("\t\tdelete(this.m_removed_rows, k)");
				code.AppendLine("\t}");
				code.AppendLine("\trows := this.fetch_rows(this.m_rows, save_all)");
                code.AppendLine("\tthis.save_rows(rows, quick)");
				code.AppendLine("\tnew_rows := this.fetch_new_rows(save_all)");
                code.AppendLine("\tthis.save_rows(new_rows, quick)");
				code.AppendLine("\tthis.m_now_save_index++");
                code.AppendLine("\treturn");
                code.AppendLine("}");

                //add row
                if (table.AutoPrimaryKey)
                {
                    code.AppendLine("func (this *" + table_name + ") AddRow() (row *" + row_name + ") {");
                }
                else
                {
                    code.AppendLine("func (this *" + table_name + ") AddRow(" + table.PrimaryKeyName + " " + table.PrimaryKeyType + ") (row *" + row_name + ") {");
                }
                if (gc)
				{
					//code.AppendLine("\tthis.GC()");
					if (!table.AutoPrimaryKey)
					{
						code.AppendLine("\tthis.GCRow(" + table.PrimaryKeyName + ")");
					}
                }
                code.AppendLine("\tthis.m_lock.UnSafeLock(\"" + table_name + ".AddRow\")");
                code.AppendLine("\tdefer this.m_lock.UnSafeUnlock()");
                if (table.AutoPrimaryKey)
                {
                    code.AppendLine("\t" + table.PrimaryKeyName + " := atomic.AddInt32(&this.m_max_id, 1)");
                    code.AppendLine("\tthis.m_max_id_changed = true");
                }
                code.AppendLine("\trow = new_" + row_name + "(this," + table.PrimaryKeyName + ")");
                code.AppendLine("\trow.m_new = true");
                code.AppendLine("\trow.m_loaded = true");
                code.AppendLine("\trow.m_valid = true");
                if (!table.AutoPrimaryKey)
                {
                    code.AppendLine("\t_, has := this.m_new_rows[" + table.PrimaryKeyName + "]");
                    code.AppendLine("\tif has{");
                    code.AppendLine("\t\tlog.Error(\"已经存在 %v\", " + table.PrimaryKeyName + ")");
                    code.AppendLine("\t\treturn nil");
                    code.AppendLine("\t}");
                }
				code.AppendLine("\trow.m_save_index = int32(this.m_save_count % config.DBCST_MIN)");
				code.AppendLine("\tthis.m_save_count++");
                code.AppendLine("\tthis.m_new_rows[" + table.PrimaryKeyName + "] = row");
                code.AppendLine("\tatomic.AddInt32(&this.m_gc_n,1)");
                code.AppendLine("\treturn row");
                code.AppendLine("}");

                //remove row
                code.AppendLine("func (this *" + table_name + ") RemoveRow(" + table.PrimaryKeyName + " " + table.PrimaryKeyType + ") {");
                code.AppendLine("\tthis.m_lock.UnSafeLock(\"" + table_name + ".RemoveRow\")");
                code.AppendLine("\tdefer this.m_lock.UnSafeUnlock()");
                code.AppendLine("\trow := this.m_rows[" + table.PrimaryKeyName + "]");
                code.AppendLine("\tif row != nil {");
                code.AppendLine("\t\trow.m_remove = true");
                code.AppendLine("\t\tdelete(this.m_rows, " + table.PrimaryKeyName + ")");
				code.AppendLine("\t\trm_row := this.m_removed_rows[" + table.PrimaryKeyName + "]");
				code.AppendLine("\t\tif rm_row != nil {");
                code.AppendLine("\t\t\tlog.Error(\"rows and removed rows both has %v\", " + table.PrimaryKeyName + ")");
                code.AppendLine("\t\t}");
				code.AppendLine("\t\tthis.m_removed_rows[" + table.PrimaryKeyName + "] = row");
                code.AppendLine("\t\t_, has_new := this.m_new_rows[" + table.PrimaryKeyName + "]");
                code.AppendLine("\t\tif has_new {");
                code.AppendLine("\t\t\tdelete(this.m_new_rows, " + table.PrimaryKeyName + ")");
                code.AppendLine("\t\t\tlog.Error(\"rows and new_rows both has %v\", " + table.PrimaryKeyName + ")");
                code.AppendLine("\t\t}");
                code.AppendLine("\t} else {");
                code.AppendLine("\t\trow = this.m_removed_rows[" + table.PrimaryKeyName + "]");
                code.AppendLine("\t\tif row == nil {");
                code.AppendLine("\t\t\t_, has_new := this.m_new_rows[" + table.PrimaryKeyName + "]");
                code.AppendLine("\t\t\tif has_new {");
                code.AppendLine("\t\t\t\tdelete(this.m_new_rows, " + table.PrimaryKeyName + ")");
                code.AppendLine("\t\t\t} else {");
                code.AppendLine("\t\t\t\tlog.Error(\"row not exist %v\", " + table.PrimaryKeyName + ")");
                code.AppendLine("\t\t\t}");
                code.AppendLine("\t\t} else {");
                code.AppendLine("\t\t\tlog.Error(\"already removed %v\", " + table.PrimaryKeyName + ")");
                code.AppendLine("\t\t\t_, has_new := this.m_new_rows[" + table.PrimaryKeyName + "]");
                code.AppendLine("\t\t\tif has_new {");
                code.AppendLine("\t\t\t\tdelete(this.m_new_rows, " + table.PrimaryKeyName + ")");
                code.AppendLine("\t\t\t\tlog.Error(\"removed rows and new_rows both has %v\", " + table.PrimaryKeyName + ")");
                code.AppendLine("\t\t\t}");
                code.AppendLine("\t\t}");
                code.AppendLine("\t}");
                code.AppendLine("}");

                //get row
                code.AppendLine("func (this *" + table_name + ") GetRow("+table.PrimaryKeyName+" "+table.PrimaryKeyType+") (row *"+row_name+") {");
                code.AppendLine("\tthis.m_lock.UnSafeRLock(\"" + table_name + ".GetRow\")");
                code.AppendLine("\tdefer this.m_lock.UnSafeRUnlock()");
                code.AppendLine("\trow = this.m_rows[" + table.PrimaryKeyName + "]");
                code.AppendLine("\tif row == nil {");
                code.AppendLine("\t\trow = this.m_new_rows[" + table.PrimaryKeyName + "]");
                code.AppendLine("\t}");
                code.AppendLine("\treturn row");
                code.AppendLine("}");

                //gc
                if (gc)
                {
                    //size
                    code.AppendLine("func (this *" + table_name + ") SetPoolSize(n int32) {");
                    code.AppendLine("\tthis.m_pool_size = n");
                    code.AppendLine("}");

					code.AppendLine("func (this *" + table_name + ") GCRow(" + table.PrimaryKeyName + " " + table.PrimaryKeyType + ") {");
					code.AppendLine("\tif !atomic.CompareAndSwapInt32(&this.m_gcing, 0, 1) {");
					code.AppendLine("\t\treturn");
					code.AppendLine("\t}");
					code.AppendLine("\tdefer atomic.StoreInt32(&this.m_gcing, 0)");
					code.AppendLine("\trow := this.m_rows[" + table.PrimaryKeyName + "]");
					code.AppendLine("\tif nil == row {");
					code.AppendLine("\t\treturn");
					code.AppendLine("\t}");
					code.AppendLine("\trow.SetSaveRelease(true)");
					code.AppendLine("\terr, _, _ := row.Save(true)");
					code.AppendLine("\tif err != nil {");
					code.AppendLine("\t\tlog.Error(\"" + table_name + " row %v release failed %v\", " + table.PrimaryKeyName + ", err)");
					code.AppendLine("\t}");
					code.AppendLine("\treturn");
					code.AppendLine("}");

                    //gc
                    code.AppendLine("func (this *" + table_name + ") GC() {");
                    code.AppendLine("\tif this.m_pool_size<=0{");
                    code.AppendLine("\t\treturn");
                    code.AppendLine("\t}");
                    code.AppendLine("\tif !atomic.CompareAndSwapInt32(&this.m_gcing, 0, 1) {");
                    code.AppendLine("\t\treturn");
                    code.AppendLine("\t}");
                    code.AppendLine("\tdefer atomic.StoreInt32(&this.m_gcing, 0)");
                    code.AppendLine("\tn := atomic.LoadInt32(&this.m_gc_n)");
                    code.AppendLine("\tif float32(n) < float32(this.m_pool_size)*1.2 {");
                    code.AppendLine("\t\treturn");
                    code.AppendLine("\t}");
                    code.AppendLine("\tmax := (n - this.m_pool_size) / 2");
                    code.AppendLine("\tarr := " + row_name + "Sort{}");
                    code.AppendLine("\trows := this.fetch_rows(this.m_rows, true)");
                    code.AppendLine("\tarr.rows = make([]*" + row_name + ", len(rows))");
                    code.AppendLine("\tindex := 0");
                    code.AppendLine("\tfor _, v := range rows {");
                    code.AppendLine("\t\tarr.rows[index] = v");
                    code.AppendLine("\t\tindex++");
                    code.AppendLine("\t}");
                    code.AppendLine("\tsort.Sort(&arr)");
                    code.AppendLine("\tcount := int32(0)");
                    code.AppendLine("\tfor _, v := range arr.rows {");
					code.AppendLine("\t\tv.SetSaveRelease(true)");
                    code.AppendLine("\t\terr, _, released := v.Save(true)");
                    code.AppendLine("\t\tif err != nil {");
                    code.AppendLine("\t\t\tlog.Error(\"release failed %v\", err)");
                    code.AppendLine("\t\t\tcontinue");
                    code.AppendLine("\t\t}");
                    code.AppendLine("\t\tif released {");
                    code.AppendLine("\t\t\tcount++");
                    code.AppendLine("\t\t\tif count > max {");
                    code.AppendLine("\t\t\t\treturn");
                    code.AppendLine("\t\t\t}");
                    code.AppendLine("\t\t}");
                    code.AppendLine("\t}");
                    code.AppendLine("\treturn");
                    code.AppendLine("}");
                }
            }

            #endregion
        }

        public static void Compile(string lang, string def_file, string proto_pfefix_file, string proto_file, string code_prefix_file, string code_file)
        {
            var ser = new DataContractJsonSerializer(typeof(DbDefinition));
            FileStream fs = File.Open(def_file, FileMode.Open, FileAccess.Read);
            var def = ser.ReadObject(fs) as DbDefinition;
            fs.Close();

            //check name unique
            CheckTableNamesUnique(def.Tables);

            #region proto
            StringBuilder proto = new StringBuilder();

            //namespace
            proto.AppendLine("package " + def.Namespace + ";");
            proto.AppendLine();

            //proto prefix
            string proto_prefix = File.ReadAllText(proto_pfefix_file);
            proto.Append(proto_prefix);
            proto.AppendLine();

            foreach (var s in def.Structs)
            {
                BuildProto(proto, s.Name, s.Fields, false);
            }

            foreach (var t in def.Tables)
            {
                foreach (var c in t.Columns)
                {
                    if (c.Simple)
                    {
                        continue;
                    }

                    BuildProto(proto, t.TableName + c.ColumnName, c.Fields, c.Map);
                }
            }

            CreateFile(proto_file, proto.ToString());
            #endregion

            #region code
            StringBuilder code = new StringBuilder();
            //code prefix
            string code_prefix = File.ReadAllText(code_prefix_file);
            code.Append(code_prefix);
            code.AppendLine();
            code.AppendLine();

            //version
            code.AppendLine("const DBC_VERSION = " + def.Version);
            code.AppendLine("const DBC_SUB_VERSION = " + def.SubVersion);
            code.AppendLine();
            
            //data defs
            StringBuilder struct_def = new StringBuilder();
            foreach (var s in def.Structs)
            {
                BuildStruct(struct_def, s.Name, s.Fields);
            }
            foreach (var t in def.Tables)
            {
                foreach (var c in t.Columns)
                {
                    if (c.Simple)
                    {
                        continue;
                    }

                    BuildStruct(struct_def, t.TableName + c.ColumnName, c.Fields);
                }
            }
            code.Append(struct_def);
            code.AppendLine();
            
            //tables
            foreach (var t in def.Tables)
            {
                BuildTable(t, code);
            }
            code.AppendLine();

            //dbc def
            code.AppendLine("type DBC struct {");
            code.AppendLine("\tm_db_name            string");
            code.AppendLine("\tm_db                 *sql.DB");
            code.AppendLine("\tm_db_lock            *Mutex");
            code.AppendLine("\tm_initialized        bool");
            code.AppendLine("\tm_quit               bool");
            code.AppendLine("\tm_shutdown_completed bool");
            code.AppendLine("\tm_shutdown_lock      *Mutex");
			//code.AppendLine("\tm_db_last_copy_time	int32");
			//code.AppendLine("\tm_db_copy_path		string");
			//code.AppendLine("\tm_db_addr			string");
			//code.AppendLine("\tm_db_account			string");
			//code.AppendLine("\tm_db_password		string");
            foreach (var t in def.Tables)
            {
                if (t.SingleRow)
                {
                    code.AppendLine("\t"+t.TableName+" *db"+t.TableName+"Table");
                }
                else
                {
                    code.AppendLine("\t" + t.TableName + "s *db" + t.TableName + "Table");
                }               
            }
            code.AppendLine("}");

            //dbc init table
            code.AppendLine("func (this *DBC)init_tables()(err error){");
            foreach (var t in def.Tables)
            {
                string member_name = t.SingleRow ? t.TableName : t.TableName + "s";
                code.AppendLine("\tthis." + member_name + " = new_db" + t.TableName + "Table(this)");
                code.AppendLine("\terr = this." + member_name + ".Init()");
                code.AppendLine("\tif err != nil {");
                code.AppendLine("\t\tlog.Error(\"init " + member_name + " table failed\")");
                code.AppendLine("\t\treturn");
                code.AppendLine("\t}");
            }
            code.AppendLine("\treturn");
            code.AppendLine("}");

            //dbc preload
            code.AppendLine("func (this *DBC)Preload()(err error){");
            foreach (var t in def.Tables)
            {
                string member_name = t.SingleRow ? t.TableName : t.TableName + "s";
                code.AppendLine("\terr = this." + member_name + ".Preload()");
                code.AppendLine("\tif err != nil {");
                code.AppendLine("\t\tlog.Error(\"preload " + member_name + " table failed\")");
                code.AppendLine("\t\treturn");
                code.AppendLine("\t}else{");
                code.AppendLine("\t\tlog.Info(\"preload " + member_name + " table succeed !\")");
                code.AppendLine("\t}");
            }
            code.AppendLine("\terr = this.on_preload()");
            code.AppendLine("\tif err != nil {");
            code.AppendLine("\t\tlog.Error(\"on_preload failed\")");
            code.AppendLine("\t\treturn");
            code.AppendLine("\t}");
            code.AppendLine("\terr = this.Save(true, true)");
            code.AppendLine("\tif err != nil {");
            code.AppendLine("\t\tlog.Error(\"save on preload failed\")");
            code.AppendLine("\t\treturn");
            code.AppendLine("\t}");
            code.AppendLine("\treturn");
            code.AppendLine("}");

            //dbc save
            code.AppendLine("func (this *DBC)Save(quick bool, save_all bool)(err error){");
            foreach (var t in def.Tables)
            {
                string member_name = t.SingleRow ? t.TableName : t.TableName + "s";
				code.AppendLine("\terr = this." + member_name + ".Save(quick, save_all)");
                code.AppendLine("\tif err != nil {");
                code.AppendLine("\t\tlog.Error(\"save " + member_name + " table failed\")");
                code.AppendLine("\t\treturn");
                code.AppendLine("\t}");
            }
            code.AppendLine("\treturn");
            code.AppendLine("}");

            //output
            CreateFile(code_file, code.ToString());
            #endregion
        }
    }

    class Program
    {
        static void Main(string[] args)
        {
            try
            {
                if (File.Exists("dbc.log"))
                {
                    File.Delete("dbc.log");
                }

                //args = new string[] { "go", @"E:\gos\test4\Server\src\db\db_def_sql.json", @"E:\gos\test4\Server\src\db\proto_prefix.txt", @"E:\gos\test4\Server\src\public_message\db.proto", @"E:\gos\test4\Server\src\db\code_prefix.txt", @"E:\gos\test4\Server\src\youma\game_server\db_tables.go" };

                if (args == null)
                {
                    throw new ArgumentNullException("命令行不能为空");
                }
                if (args.Length != 6)
                {
                    throw new ArgumentException("命令行参数数量不正确");
                }

                string lang = args[0];
                string def_file = args[1];
                string proto_prefix = args[2];
                string proto_file = args[3];
                string code_prefix = args[4];
                string code_file = args[5];

                if (lang != "go")
                {
                    throw new ApplicationException("不支持的语言 " + lang);
                }

                DbCompiler.Compile(lang, def_file, proto_prefix, proto_file, code_prefix, code_file);
            }
            catch (System.Exception ex)
            {
                File.AppendAllText("dbc.log", ex.Message + "\n" + ex.StackTrace + "\n");
            }

        }
    }
}